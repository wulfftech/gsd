package reward

import (
	"strconv"

	auth "donetick.com/core/internal/auth"
	cRepo "donetick.com/core/internal/circle/repo"
	rModel "donetick.com/core/internal/reward/model"
	rRepo "donetick.com/core/internal/reward/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	rRepo      *rRepo.RewardRepository
	circleRepo *cRepo.CircleRepository
}

func NewHandler(rRepo *rRepo.RewardRepository, circleRepo *cRepo.CircleRepository) *Handler {
	return &Handler{
		rRepo:      rRepo,
		circleRepo: circleRepo,
	}
}

func (h *Handler) isAdmin(c *gin.Context, circleID int, userID int) (bool, error) {
	admins, err := h.circleRepo.GetCircleAdmins(c, circleID)
	if err != nil {
		return false, err
	}
	for _, a := range admins {
		if a.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) getRewards(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}
	rewards, err := h.rRepo.GetRewards(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting rewards"})
		return
	}
	c.JSON(200, rewards)
}

func (h *Handler) createReward(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	isAdmin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("Error checking admin status:", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Points      int    `json:"points" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	reward := &rModel.Reward{
		CircleID:    currentUser.CircleID,
		Name:        req.Name,
		Description: req.Description,
		Points:      req.Points,
		CreatedBy:   currentUser.ID,
		IsActive:    true,
	}
	if err := h.rRepo.CreateReward(c, reward); err != nil {
		log.Error("Error creating reward:", err)
		c.JSON(500, gin.H{"error": "Error creating reward"})
		return
	}
	c.JSON(200, gin.H{"res": reward})
}

func (h *Handler) updateReward(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	isAdmin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("Error checking admin status:", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}

	rewardIDRaw := c.Param("id")
	rewardID, err := strconv.Atoi(rewardIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Points      int    `json:"points" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	reward := &rModel.Reward{
		ID:          rewardID,
		CircleID:    currentUser.CircleID,
		Name:        req.Name,
		Description: req.Description,
		Points:      req.Points,
	}
	if err := h.rRepo.UpdateReward(c, reward); err != nil {
		log.Error("Error updating reward:", err)
		c.JSON(500, gin.H{"error": "Error updating reward"})
		return
	}
	c.JSON(200, gin.H{"res": reward})
}

func (h *Handler) deleteReward(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	isAdmin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("Error checking admin status:", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}

	rewardIDRaw := c.Param("id")
	rewardID, err := strconv.Atoi(rewardIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	if err := h.rRepo.DeleteReward(c, currentUser.CircleID, rewardID); err != nil {
		log.Error("Error deleting reward:", err)
		c.JSON(500, gin.H{"error": "Error deleting reward"})
		return
	}
	c.JSON(200, gin.H{"res": "Reward deleted"})
}

func (h *Handler) redeemReward(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	rewardIDRaw := c.Param("id")
	rewardID, err := strconv.Atoi(rewardIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	reward, err := h.rRepo.GetRewardByID(c, currentUser.CircleID, rewardID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Reward not found"})
		return
	}

	if err := h.rRepo.RedeemReward(c, currentUser.CircleID, currentUser.ID, reward); err != nil {
		if err.Error() == "insufficient points" {
			c.JSON(400, gin.H{"error": "You do not have enough points to redeem this reward"})
			return
		}
		log.Error("Error redeeming reward:", err)
		c.JSON(500, gin.H{"error": "Error redeeming reward"})
		return
	}
	c.JSON(200, gin.H{"res": "Reward redeemed successfully"})
}

func (h *Handler) getRedemptions(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	isAdmin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("Error checking admin status:", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}

	redemptions, err := h.rRepo.GetRedemptions(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting redemptions:", err)
		c.JSON(500, gin.H{"error": "Error getting redemptions"})
		return
	}
	c.JSON(200, redemptions)
}

func (h *Handler) fulfillRedemption(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{"error": "Error getting current user"})
		return
	}

	isAdmin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("Error checking admin status:", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}

	redemptionIDRaw := c.Param("id")
	redemptionID, err := strconv.Atoi(redemptionIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid redemption ID"})
		return
	}

	if err := h.rRepo.FulfillRedemption(c, currentUser.CircleID, redemptionID); err != nil {
		log.Error("Error fulfilling redemption:", err)
		c.JSON(500, gin.H{"error": "Error fulfilling redemption"})
		return
	}
	c.JSON(200, gin.H{"res": "Redemption fulfilled"})
}

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {
	rewardRoutes := r.Group("api/v1/rewards")
	rewardRoutes.Use(auth.MiddlewareFunc())
	{
		rewardRoutes.GET("", h.getRewards)
		rewardRoutes.POST("", h.createReward)
		rewardRoutes.PUT("/:id", h.updateReward)
		rewardRoutes.DELETE("/:id", h.deleteReward)
		rewardRoutes.POST("/:id/redeem", h.redeemReward)
		rewardRoutes.GET("/redemptions", h.getRedemptions)
		rewardRoutes.PUT("/redemptions/:id/fulfill", h.fulfillRedemption)
	}
}
