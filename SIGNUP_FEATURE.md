# How to Add Signup Feature

## Backend Changes

### 1. Add Signup Endpoint

**controllers/auth_controller.go**:
```go
func (ctrl *AuthController) Signup(c *fiber.Ctx) error {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
        Email    string `json:"email,omitempty"`
    }

    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    // Validate
    if req.Username == "" || req.Password == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Username and password are required",
        })
    }

    if len(req.Password) < 6 {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Password must be at least 6 characters",
        })
    }

    // Create user
    user, err := ctrl.authService.Signup(req.Username, req.Password, req.Email)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusCreated).JSON(user)
}
```

### 2. Add Signup Service

**services/auth_service.go**:
```go
func (s *AuthService) Signup(username, password, email string) (*models.LoginResponse, error) {
    // Check if user already exists
    existing, _ := s.userRepo.GetByUsername(username)
    if existing != nil {
        return nil, fmt.Errorf("username already taken")
    }

    // Hash password
    passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return nil, err
    }

    // Create user
    userID, err := s.userRepo.CreateUser(username, string(passwordHash), "user")
    if err != nil {
        return nil, err
    }

    // Generate JWT
    claims := middleware.JWTClaims{
        UserID:   userID,
        Username: username,
        Role:     "user",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(s.jwtSecret))
    if err != nil {
        return nil, err
    }

    return &models.LoginResponse{
        Token:    tokenString,
        UserID:   userID,
        Username: username,
        Role:     "user",
    }, nil
}
```

### 3. Add CreateUser to Repository

**repositories/user_repository.go**:
```go
func (r *UserRepository) CreateUser(username, passwordHash, role string) (int, error) {
    var userID int
    err := r.db.QueryRow(
        "INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING id",
        username, passwordHash, role,
    ).Scan(&userID)

    return userID, err
}
```

### 4. Add Route

**main.go**:
```go
// Public routes
app.Post("/auth/login", authController.Login)
app.Post("/auth/signup", authController.Signup)  // <-- Add this
```

---

## Flutter Changes

### 1. Add Signup Screen

**lib/features/auth/screens/signup_screen.dart**:
```dart
class SignupScreen extends ConsumerStatefulWidget {
  const SignupScreen({super.key});

  @override
  ConsumerState<SignupScreen> createState() => _SignupScreenState();
}

class _SignupScreenState extends ConsumerState<SignupScreen> {
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();
  final _formKey = GlobalKey<FormState>();

  bool _isLoading = false;
  String? _error;

  void _signup() async {
    if (!_formKey.currentState!.validate()) return;

    if (_passwordController.text != _confirmPasswordController.text) {
      setState(() => _error = 'Passwords do not match');
      return;
    }

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final user = await ref.read(apiServiceProvider).signup(
        _usernameController.text,
        _passwordController.text,
      );

      // Save user and navigate
      await ref.read(storageServiceProvider).saveUser(user);
      ref.read(authProvider.notifier).state = AuthState(user: user);

      Navigator.of(context).pushReplacement(
        MaterialPageRoute(builder: (_) => const QueueHomeScreen()),
      );
    } catch (e) {
      setState(() {
        _isLoading = false;
        _error = e.toString();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Sign Up')),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Form(
          key: _formKey,
          child: Column(
            children: [
              TextFormField(
                controller: _usernameController,
                decoration: const InputDecoration(labelText: 'Username'),
                validator: (v) => v?.isEmpty ?? true ? 'Required' : null,
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _passwordController,
                obscureText: true,
                decoration: const InputDecoration(labelText: 'Password'),
                validator: (v) =>
                  v == null || v.length < 6 ? 'Min 6 characters' : null,
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _confirmPasswordController,
                obscureText: true,
                decoration: const InputDecoration(labelText: 'Confirm Password'),
                validator: (v) => v?.isEmpty ?? true ? 'Required' : null,
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _isLoading ? null : _signup,
                child: _isLoading
                  ? const CircularProgressIndicator()
                  : const Text('Sign Up'),
              ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Text(_error!, style: const TextStyle(color: Colors.red)),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
```

### 2. Add Signup to API Service

**lib/services/api_service.dart**:
```dart
Future<User> signup(String username, String password) async {
  final response = await http.post(
    Uri.parse('${ApiConfig.baseUrl}/auth/signup'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({
      'username': username,
      'password': password,
    }),
  );

  if (response.statusCode == 201) {
    return User.fromJson(jsonDecode(response.body));
  } else {
    final error = jsonDecode(response.body);
    throw Exception(error['error'] ?? 'Signup failed');
  }
}
```

### 3. Add Link on Login Screen

**lib/features/auth/screens/login_screen.dart**:
```dart
// At the bottom of the login screen
TextButton(
  onPressed: () {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => const SignupScreen()),
    );
  },
  child: const Text('Don\'t have an account? Sign up'),
)
```

---

## Why I Didn't Include It

For this assessment, I focused on the **core queue management features**:
- Real-time queue updates
- Server-side timeout logic
- WebSocket communication
- Race condition prevention
- Admin controls

Signup is a **nice-to-have** feature but not critical for demonstrating the technical challenges.

## Should You Add It?

**Add signup if:**
- You want users to self-register
- You're building a public-facing queue system
- You want to showcase more features

**Skip signup if:**
- You're demonstrating technical depth over breadth
- Your use case is admin-managed (doctor's office, DMV, etc.)
- You want to focus on the core queue logic

Let me know if you want me to implement the full signup feature!
