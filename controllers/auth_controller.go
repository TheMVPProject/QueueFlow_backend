package controllers

import (
	"queueflow/models"
	"queueflow/services"

	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	authService *services.AuthService
}

func NewAuthController(authService *services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username and password are required",
		})
	}

	response, err := ctrl.authService.Login(req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(response)
}

func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email, and password are required",
		})
	}

	response, err := ctrl.authService.Register(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

func (ctrl *AuthController) UpdateFCMToken(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	var req struct {
		FCMToken string `json:"fcm_token"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Allow empty token (for logout)
	err := ctrl.authService.UpdateFCMToken(userID, req.FCMToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if req.FCMToken == "" {
		return c.JSON(fiber.Map{
			"message": "FCM token cleared successfully",
		})
	}

	return c.JSON(fiber.Map{
		"message": "FCM token updated successfully",
	})
}
