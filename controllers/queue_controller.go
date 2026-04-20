package controllers

import (
	"queueflow/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type QueueController struct {
	queueService *services.QueueService
}

func NewQueueController(queueService *services.QueueService) *QueueController {
	return &QueueController{queueService: queueService}
}

func (ctrl *QueueController) JoinQueue(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	entry, err := ctrl.queueService.JoinQueue(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Successfully joined the queue",
		"entry":    entry,
		"position": entry.Position,
	})
}

func (ctrl *QueueController) LeaveQueue(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	err := ctrl.queueService.LeaveQueue(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Successfully left the queue",
	})
}

func (ctrl *QueueController) ConfirmTurn(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	entry, err := ctrl.queueService.ConfirmTurn(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Turn confirmed successfully",
		"entry":   entry,
	})
}

func (ctrl *QueueController) GetQueueStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	status, err := ctrl.queueService.GetUserQueueStatus(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if status == nil {
		return c.JSON(fiber.Map{
			"in_queue": false,
			"message":  "You are not in the queue",
		})
	}

	return c.JSON(fiber.Map{
		"in_queue": true,
		"status":   status,
	})
}

func (ctrl *QueueController) GetQueueList(c *fiber.Ctx) error {
	queue, err := ctrl.queueService.GetQueueList()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"queue": queue,
		"total": len(queue),
	})
}

// Admin endpoints

func (ctrl *QueueController) CallNext(c *fiber.Ctx) error {
	entry, err := ctrl.queueService.CallNextUser()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "User called successfully",
		"entry":   entry,
	})
}

func (ctrl *QueueController) RemoveUser(c *fiber.Ctx) error {
	userIDStr := c.Params("user_id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	err = ctrl.queueService.RemoveUser(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "User removed from queue successfully",
	})
}

func (ctrl *QueueController) PauseQueue(c *fiber.Ctx) error {
	err := ctrl.queueService.PauseQueue()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Queue paused successfully",
	})
}

func (ctrl *QueueController) ResumeQueue(c *fiber.Ctx) error {
	err := ctrl.queueService.ResumeQueue()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Queue resumed successfully",
	})
}
