package project

import (
	"strconv"
	"time"

	auth "donetick.com/core/internal/auth"
	cRepo "donetick.com/core/internal/circle/repo"
	projModel "donetick.com/core/internal/project/model"
	pRepo "donetick.com/core/internal/project/repo"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	pRepo      *pRepo.ProjectRepository
	circleRepo *cRepo.CircleRepository
}

func NewHandler(pRepo *pRepo.ProjectRepository, circleRepo *cRepo.CircleRepository) *Handler {
	return &Handler{pRepo: pRepo, circleRepo: circleRepo}
}

// isAdmin returns true if userID has the admin role in circleID.
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

// getProjects returns all projects in the user's circle (with tasks + assignees).
func (h *Handler) getProjects(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	projects, err := h.pRepo.GetCircleProjectsWithDetails(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting projects"})
		return
	}
	c.JSON(200, projects)
}

// getProject returns a single project by ID.
func (h *Handler) getProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
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

// createProject is admin-only.
func (h *Handler) createProject(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("isAdmin check failed", "err", err)
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can create projects"})
		return
	}

	var req projModel.ProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	project := &projModel.Project{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		CircleID:    currentUser.CircleID,
		CreatedBy:   currentUser.ID,
		Points:      req.Points,
	}
	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format (expected RFC3339)"})
			return
		}
		project.DueDate = &t
	}

	if err := h.pRepo.CreateProject(c, project, req.AssigneeIDs, req.Tasks); err != nil {
		log.Error("create project failed", "err", err)
		c.JSON(500, gin.H{"error": "Error creating project"})
		return
	}

	// Reload with relations
	full, err := h.pRepo.GetProjectByID(c, project.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(200, gin.H{"res": project})
		return
	}
	c.JSON(200, gin.H{"res": full})
}

// updateProject is admin-only.
func (h *Handler) updateProject(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can edit projects"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}

	var req projModel.ProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	project := &projModel.Project{
		ID:          projectID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		Points:      req.Points,
	}
	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format"})
			return
		}
		project.DueDate = &t
	}

	if err := h.pRepo.UpdateProject(c, project, req.AssigneeIDs, req.Tasks, currentUser.CircleID); err != nil {
		log.Error("update project failed", "err", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	full, err := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error fetching updated project"})
		return
	}
	c.JSON(200, gin.H{"res": full})
}

// deleteProject is admin-only.
func (h *Handler) deleteProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can delete projects"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}

	if err := h.pRepo.DeleteProject(c, projectID, currentUser.CircleID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"res": "Project deleted"})
}

// markTaskDone is available to any project assignee.
func (h *Handler) markTaskDone(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}
	taskID, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}

	// Admins can also mark tasks done; otherwise user must be an assignee
	admin, _ := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if !admin {
		isAssignee, err := h.pRepo.IsAssignee(c, projectID, currentUser.ID)
		if err != nil || !isAssignee {
			c.JSON(403, gin.H{"error": "Only project assignees can mark tasks done"})
			return
		}
	}

	if err := h.pRepo.MarkTaskDone(c, taskID, projectID, currentUser.ID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"res": "Task submitted for approval"})
}

// approveTask is admin-only. Completing the last task triggers project completion + points.
func (h *Handler) approveTask(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can approve tasks"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}
	taskID, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}

	completedProject, err := h.pRepo.ApproveTask(c, taskID, projectID, currentUser.CircleID, currentUser.ID)
	if err != nil {
		log.Error("approve task failed", "err", err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{"res": "Task approved"}
	if completedProject != nil {
		resp["projectCompleted"] = true
		resp["project"] = completedProject
	}
	c.JSON(200, resp)
}

// rejectTask resets the task to Pending so any assignee can resubmit.
func (h *Handler) rejectTask(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can reject tasks"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}
	taskID, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid task ID"})
		return
	}

	if err := h.pRepo.RejectTask(c, taskID, projectID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"res": "Task returned for rework"})
}

// reopenProject resets a completed project back to active (all tasks become Pending).
func (h *Handler) reopenProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	admin, err := h.isAdmin(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !admin {
		c.JSON(403, gin.H{"error": "Only admins can reopen projects"})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}

	if err := h.pRepo.ReopenProject(c, projectID, currentUser.CircleID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"res": "Project reopened"})
}

func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	projectRoutes := r.Group("api/v1/projects")
	projectRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		projectRoutes.GET("", h.getProjects)
		projectRoutes.GET("/:id", h.getProject)
		projectRoutes.POST("", h.createProject)
		projectRoutes.PUT("/:id", h.updateProject)
		projectRoutes.DELETE("/:id", h.deleteProject)

		projectRoutes.POST("/:id/tasks/:taskId/done", h.markTaskDone)
		projectRoutes.POST("/:id/tasks/:taskId/approve", h.approveTask)
		projectRoutes.POST("/:id/tasks/:taskId/reject", h.rejectTask)
		projectRoutes.POST("/:id/reopen", h.reopenProject)
	}
}
