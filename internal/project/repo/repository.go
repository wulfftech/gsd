package repo

import (
	"context"
	"errors"
	"time"

	config "donetick.com/core/config"
	ptsModel "donetick.com/core/internal/points"
	projModel "donetick.com/core/internal/project/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB, cfg *config.Config) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) GetCircleProjectsWithDetails(ctx context.Context, circleID int) ([]*projModel.Project, error) {
	var projects []*projModel.Project
	err := r.db.WithContext(ctx).
		Where("circle_id = ?", circleID).
		Order("created_at DESC").
		Preload("Assignees").
		Preload("Tasks", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		Find(&projects).Error
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Assignees == nil {
			p.Assignees = make([]projModel.ProjectAssignee, 0)
		}
		if p.Tasks == nil {
			p.Tasks = make([]projModel.ProjectTask, 0)
		}
	}
	return projects, nil
}

func (r *ProjectRepository) GetProjectByID(ctx context.Context, projectID int, circleID int) (*projModel.Project, error) {
	var project projModel.Project
	err := r.db.WithContext(ctx).
		Where("id = ? AND circle_id = ?", projectID, circleID).
		Preload("Assignees").
		Preload("Tasks", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		First(&project).Error
	if err != nil {
		return nil, err
	}
	if project.Assignees == nil {
		project.Assignees = make([]projModel.ProjectAssignee, 0)
	}
	if project.Tasks == nil {
		project.Tasks = make([]projModel.ProjectTask, 0)
	}
	return &project, nil
}

func (r *ProjectRepository) CreateProject(ctx context.Context, project *projModel.Project, assigneeIDs []int, tasks []projModel.ProjectTaskReq) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(project).Error; err != nil {
			return err
		}
		for _, uid := range assigneeIDs {
			a := projModel.ProjectAssignee{ProjectID: project.ID, UserID: uid}
			if err := tx.Create(&a).Error; err != nil {
				return err
			}
		}
		for i, t := range tasks {
			task := projModel.ProjectTask{
				ProjectID:   project.ID,
				Name:        t.Name,
				Description: t.Description,
				SortOrder:   i,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ProjectRepository) UpdateProject(ctx context.Context, project *projModel.Project, assigneeIDs []int, taskReqs []projModel.ProjectTaskReq, circleID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing projModel.Project
		if err := tx.Where("id = ? AND circle_id = ?", project.ID, circleID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("project not found")
			}
			return err
		}

		updates := map[string]interface{}{
			"name":        project.Name,
			"description": project.Description,
			"color":       project.Color,
			"icon":        project.Icon,
			"due_date":    project.DueDate,
			"points":      project.Points,
		}
		if err := tx.Model(&projModel.Project{}).Where("id = ?", project.ID).Updates(updates).Error; err != nil {
			return err
		}

		// Replace assignees entirely
		if err := tx.Where("project_id = ?", project.ID).Delete(&projModel.ProjectAssignee{}).Error; err != nil {
			return err
		}
		for _, uid := range assigneeIDs {
			a := projModel.ProjectAssignee{ProjectID: project.ID, UserID: uid}
			if err := tx.Create(&a).Error; err != nil {
				return err
			}
		}

		// Reconcile tasks:
		// - Tasks with an ID: update name/desc if still Pending; always update sort_order
		// - Tasks without an ID: create new
		// - Existing Pending tasks absent from request: delete
		var existingTasks []projModel.ProjectTask
		if err := tx.Where("project_id = ?", project.ID).Find(&existingTasks).Error; err != nil {
			return err
		}
		existingByID := make(map[int]projModel.ProjectTask, len(existingTasks))
		for _, t := range existingTasks {
			existingByID[t.ID] = t
		}

		requestedIDs := make(map[int]bool)
		for i, t := range taskReqs {
			if t.ID != nil {
				requestedIDs[*t.ID] = true
				et, ok := existingByID[*t.ID]
				if !ok {
					continue
				}
				upd := map[string]interface{}{"sort_order": i}
				if et.Status == projModel.ProjectTaskStatusPending {
					upd["name"] = t.Name
					upd["description"] = t.Description
				}
				if err := tx.Model(&projModel.ProjectTask{}).Where("id = ?", *t.ID).Updates(upd).Error; err != nil {
					return err
				}
			} else {
				task := projModel.ProjectTask{
					ProjectID:   project.ID,
					Name:        t.Name,
					Description: t.Description,
					SortOrder:   i,
				}
				if err := tx.Create(&task).Error; err != nil {
					return err
				}
			}
		}
		// Delete Pending tasks that were removed from the request
		for _, et := range existingTasks {
			if !requestedIDs[et.ID] && et.Status == projModel.ProjectTaskStatusPending {
				if err := tx.Delete(&projModel.ProjectTask{}, et.ID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *ProjectRepository) DeleteProject(ctx context.Context, projectID int, circleID int) error {
	log := logging.FromContext(ctx)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project projModel.Project
		if err := tx.Where("id = ? AND circle_id = ?", projectID, circleID).First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("project not found")
			}
			return err
		}
		if project.IsDefault {
			return errors.New("cannot delete default project")
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&projModel.ProjectTask{}).Error; err != nil {
			log.Error("error deleting project tasks", "error", err)
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&projModel.ProjectAssignee{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE chores SET project_id = NULL WHERE project_id = ?", projectID).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND circle_id = ?", projectID, circleID).Delete(&projModel.Project{}).Error
	})
}

func (r *ProjectRepository) IsAssignee(ctx context.Context, projectID int, userID int) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&projModel.ProjectAssignee{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *ProjectRepository) MarkTaskDone(ctx context.Context, taskID int, projectID int, userID int) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&projModel.ProjectTask{}).
		Where("id = ? AND project_id = ? AND status = ?", taskID, projectID, projModel.ProjectTaskStatusPending).
		Updates(map[string]interface{}{
			"status":       projModel.ProjectTaskStatusPendingApproval,
			"completed_by": userID,
			"completed_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("task not found or not in pending status")
	}
	return nil
}

func (r *ProjectRepository) ApproveTask(ctx context.Context, taskID int, projectID int, circleID int, adminID int) (*projModel.Project, error) {
	now := time.Now().UTC()
	var finalProject *projModel.Project

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&projModel.ProjectTask{}).
			Where("id = ? AND project_id = ? AND status = ?", taskID, projectID, projModel.ProjectTaskStatusPendingApproval).
			Updates(map[string]interface{}{
				"status":      projModel.ProjectTaskStatusCompleted,
				"approved_by": adminID,
				"approved_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("task not found or not awaiting approval")
		}

		var pendingCount int64
		if err := tx.Model(&projModel.ProjectTask{}).
			Where("project_id = ? AND status != ?", projectID, projModel.ProjectTaskStatusCompleted).
			Count(&pendingCount).Error; err != nil {
			return err
		}

		if pendingCount == 0 {
			proj, err := r.completeProject(tx, projectID, circleID, adminID)
			if err != nil {
				return err
			}
			finalProject = proj
		}
		return nil
	})
	return finalProject, err
}

func (r *ProjectRepository) RejectTask(ctx context.Context, taskID int, projectID int) error {
	result := r.db.WithContext(ctx).Model(&projModel.ProjectTask{}).
		Where("id = ? AND project_id = ? AND status = ?", taskID, projectID, projModel.ProjectTaskStatusPendingApproval).
		Updates(map[string]interface{}{
			"status":       projModel.ProjectTaskStatusPending,
			"completed_by": nil,
			"completed_at": nil,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("task not found or not awaiting approval")
	}
	return nil
}

func (r *ProjectRepository) ReopenProject(ctx context.Context, projectID int, circleID int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&projModel.Project{}).
			Where("id = ? AND circle_id = ?", projectID, circleID).
			Update("status", projModel.ProjectStatusActive)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("project not found")
		}
		return tx.Model(&projModel.ProjectTask{}).
			Where("project_id = ?", projectID).
			Updates(map[string]interface{}{
				"status":       projModel.ProjectTaskStatusPending,
				"completed_by": nil,
				"completed_at": nil,
				"approved_by":  nil,
				"approved_at":  nil,
			}).Error
	})
}

// completeProject awards points to all assignees and marks the project completed.
// Must be called inside an existing transaction (tx).
func (r *ProjectRepository) completeProject(tx *gorm.DB, projectID int, circleID int, adminID int) (*projModel.Project, error) {
	log := logging.DefaultLogger()
	var project projModel.Project
	if err := tx.Preload("Assignees").First(&project, projectID).Error; err != nil {
		return nil, err
	}

	if project.Points > 0 && len(project.Assignees) == 0 {
		log.Warn("completeProject: project has points but no assignees — no points will be awarded",
			"projectId", projectID, "points", project.Points)
	}

	if project.Points > 0 {
		for _, a := range project.Assignees {
			if err := tx.Exec(
				"UPDATE user_circles SET points = points + ? WHERE user_id = ? AND circle_id = ?",
				project.Points, a.UserID, circleID,
			).Error; err != nil {
				return nil, err
			}
			if err := tx.Create(&ptsModel.PointsHistory{
				Action:    ptsModel.PointsHistoryActionProject,
				CircleID:  circleID,
				UserID:    a.UserID,
				Points:    project.Points,
				CreatedAt: time.Now().UTC(),
				CreatedBy: adminID,
			}).Error; err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Model(&projModel.Project{}).Where("id = ?", projectID).
		Update("status", projModel.ProjectStatusCompleted).Error; err != nil {
		return nil, err
	}
	project.Status = projModel.ProjectStatusCompleted
	return &project, nil
}
