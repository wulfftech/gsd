package reward

import (
	"strconv"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	authMiddleware "donetick.com/core/internal/auth"
	cRepo "donetick.com/core/internal/circle/repo"
	rRepo "donetick.com/core/internal/reward/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
)

// API exposes the rewards catalog and redemption flow over the external
// (API-token) interface so Home Assistant and other integrations can read
// the catalog, redeem on behalf of a member, and fulfil redemptions.
type API struct {
	rRepo      *rRepo.RewardRepository
	circleRepo *cRepo.CircleRepository
}

func NewAPI(rRepo *rRepo.RewardRepository, circleRepo *cRepo.CircleRepository) *API {
	return &API{
		rRepo:      rRepo,
		circleRepo: circleRepo,
	}
}

func (h *API) isAdmin(c *gin.Context, circleID int, userID int) bool {
	admins, err := h.circleRepo.GetCircleAdmins(c, circleID)
	if err != nil {
		return false
	}
	for _, a := range admins {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

// GetRewards returns the reward catalog for the caller's circle.
func (h *API) GetRewards(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	rewards, err := h.rRepo.GetRewards(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting rewards"})
		return
	}
	c.JSON(200, rewards)
}

// RedeemReward redeems a reward. With no params the caller redeems for
// themselves; admins may pass ?userId=N to redeem on behalf of a member
// (so a parent's token can redeem for a child from Home Assistant).
func (h *API) RedeemReward(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser := auth.MustCurrentUser(c)

	rewardID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	redeemFor := currentUser.ID
	if userIDRaw := c.Query("userId"); userIDRaw != "" {
		userID, errParse := strconv.Atoi(userIDRaw)
		if errParse != nil {
			c.JSON(400, gin.H{"error": "Invalid userId"})
			return
		}
		if userID != currentUser.ID && !h.isAdmin(c, currentUser.CircleID, currentUser.ID) {
			c.JSON(403, gin.H{"error": "Only admins can redeem on behalf of another member"})
			return
		}
		redeemFor = userID
	}

	reward, err := h.rRepo.GetRewardByID(c, currentUser.CircleID, rewardID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Reward not found"})
		return
	}

	if err := h.rRepo.RedeemReward(c, currentUser.CircleID, redeemFor, reward); err != nil {
		if err.Error() == "insufficient points" {
			c.JSON(400, gin.H{"error": "Insufficient points to redeem this reward"})
			return
		}
		log.Error("Error redeeming reward:", err)
		c.JSON(500, gin.H{"error": "Error redeeming reward"})
		return
	}
	c.JSON(200, gin.H{"res": "Reward redeemed successfully"})
}

// GetRedemptions lists redemptions for the circle (admin/manager only).
func (h *API) GetRedemptions(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	if !h.isAdmin(c, currentUser.CircleID, currentUser.ID) {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}
	redemptions, err := h.rRepo.GetRedemptions(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting redemptions"})
		return
	}
	c.JSON(200, redemptions)
}

// FulfillRedemption marks a pending redemption fulfilled (admin/manager only).
func (h *API) FulfillRedemption(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	if !h.isAdmin(c, currentUser.CircleID, currentUser.ID) {
		c.JSON(403, gin.H{"error": "You are not an admin of this circle"})
		return
	}
	redemptionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid redemption ID"})
		return
	}
	if err := h.rRepo.FulfillRedemption(c, currentUser.CircleID, redemptionID); err != nil {
		c.JSON(500, gin.H{"error": "Error fulfilling redemption"})
		return
	}
	c.JSON(200, gin.H{"res": "Redemption fulfilled"})
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, limiter *limiter.Limiter, userRepo *uRepo.UserRepository) {
	rewardsAPI := r.Group("eapi/v1/rewards")
	rewardsAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		rewardsAPI.GET("", api.GetRewards)
		rewardsAPI.GET("/redemptions", api.GetRedemptions)
		rewardsAPI.POST("/:id/redeem", api.RedeemReward)
		rewardsAPI.PUT("/redemptions/:id/fulfill", api.FulfillRedemption)
	}
}
