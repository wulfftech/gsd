package project

import (
	"strconv"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	authMiddleware "donetick.com/core/internal/auth"
	pRepo "donetick.com/core/internal/project/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
)

// API exposes a minimal, read-only Projects surface over the external
// (API-token) interface, mirroring chore/thing/reward's eapi packages, so a
// GSD-specific dashboard (e.g. Home Assistant) can show a project's due date,
// points, and task list without going through the JWT web-app API.
//
// Deliberately read-only for this pass: no create/update/delete. Wire those
// up later behind the same token-auth + rate-limit chain if a client needs
// them.
type API struct {
	pRepo *pRepo.ProjectRepository
}

func NewAPI(pRepo *pRepo.ProjectRepository) *API {
	return &API{pRepo: pRepo}
}

// GetProjects returns all projects (with assignees + tasks) for the caller's
// circle. Reuses the same circle-scoped repo call the JWT web-app's
// getProjects handler uses.
func (h *API) GetProjects(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	projects, err := h.pRepo.GetCircleProjectsWithDetails(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting projects"})
		return
	}
	c.JSON(200, projects)
}

// GetProject returns a single project (with assignees + tasks), scoped to the
// caller's circle. GetProjectByID filters on "id = ? AND circle_id = ?" at the
// SQL level (see internal/project/repo/repository.go), so a project belonging
// to a different circle simply doesn't match and we return a plain 404 rather
// than distinguishing "not found" from "not yours" — the same cross-tenant
// safety property the rest of this package's mutating endpoints were recently
// fixed to have (see the project task reject/mark-done IDOR fix). Do not
// replace this with an "is caller an admin of some circle" check.
func (h *API) GetProject(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}
	project, err := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Project not found"})
		return
	}
	c.JSON(200, project)
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, limiter *limiter.Limiter, userRepo *uRepo.UserRepository) {
	projectsAPI := r.Group("eapi/v1/projects")
	projectsAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		projectsAPI.GET("", api.GetProjects)
		projectsAPI.GET("/:id", api.GetProject)
	}
}
