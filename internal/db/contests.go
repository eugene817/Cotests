package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

func CreateContest(database *gorm.DB, contest *Contest) error {
	if err := validateContest(contest); err != nil {
		return err
	}
	return database.Create(contest).Error
}

func ListContests(database *gorm.DB) ([]Contest, error) {
	var contests []Contest
	if err := database.Order("id DESC").Find(&contests).Error; err != nil {
		return nil, err
	}
	return contests, nil
}

func GetContest(database *gorm.DB, contestID uint) (*Contest, error) {
	var contest Contest
	if err := database.First(&contest, contestID).Error; err != nil {
		return nil, err
	}
	return &contest, nil
}

func ListPublicContests(database *gorm.DB, now time.Time) ([]Contest, error) {
	var contests []Contest
	_ = now // Discovery is separate from the submission window.
	if err := database.Where("visibility = ?", ContestPublished).
		Order("start_at ASC, id DESC").
		Find(&contests).Error; err != nil {
		return nil, err
	}
	return contests, nil
}

func GetPublicContest(database *gorm.DB, contestID uint, now time.Time) (*Contest, error) {
	var contest Contest
	_ = now // A published contest remains available as an archive after it ends.
	if err := database.Where("id = ?", contestID).
		Where("visibility = ?", ContestPublished).
		First(&contest).Error; err != nil {
		return nil, err
	}
	return &contest, nil
}

func UpdateContest(database *gorm.DB, contest *Contest) error {
	if contest.ID == 0 {
		return gorm.ErrRecordNotFound
	}
	if err := validateContest(contest); err != nil {
		return err
	}
	result := database.Model(&Contest{}).Where("id = ?", contest.ID).Updates(map[string]any{
		"title":       contest.Title,
		"description": contest.Description,
		"visibility":  contest.Visibility,
		"start_at":    contest.StartAt,
		"end_at":      contest.EndAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteContest(database *gorm.DB, contestID uint) error {
	result := database.Delete(&Contest{}, contestID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func validateContest(contest *Contest) error {
	contest.Title = strings.TrimSpace(contest.Title)
	contest.Description = strings.TrimSpace(contest.Description)
	if contest.Visibility == "" {
		contest.Visibility = ContestDraft
	}
	if contest.Title == "" {
		return ValidationError{"contest title is required"}
	}
	if len(contest.Title) > 300 || len(contest.Description) > 100000 {
		return ValidationError{"contest fields are too large"}
	}
	if contest.Visibility != ContestDraft && contest.Visibility != ContestPublished {
		return ValidationError{"invalid contest visibility"}
	}
	if contest.StartAt != nil && contest.EndAt != nil && !contest.EndAt.After(*contest.StartAt) {
		return ValidationError{"contest end time must be after start time"}
	}
	return nil
}

func CreateSeries(database *gorm.DB, series *Series) error {
	series.Title = strings.TrimSpace(series.Title)
	if series.ContestID == 0 {
		return fmt.Errorf("contest is required")
	}
	if series.Title == "" {
		return fmt.Errorf("series title is required")
	}
	if series.Position < 0 {
		return fmt.Errorf("series position must not be negative")
	}
	return database.Create(series).Error
}

func ListSeries(database *gorm.DB, contestID uint) ([]Series, error) {
	var series []Series
	if err := database.Where("contest_id = ?", contestID).Order("position ASC").Find(&series).Error; err != nil {
		return nil, err
	}
	return series, nil
}

func GetSeries(database *gorm.DB, seriesID uint) (*Series, error) {
	var series Series
	if err := database.First(&series, seriesID).Error; err != nil {
		return nil, err
	}
	return &series, nil
}

func UpdateSeries(database *gorm.DB, series *Series) error {
	if series.ID == 0 {
		return gorm.ErrRecordNotFound
	}
	series.Title = strings.TrimSpace(series.Title)
	if series.Title == "" {
		return fmt.Errorf("series title is required")
	}
	if series.Position < 0 {
		return fmt.Errorf("series position must not be negative")
	}
	result := database.Model(&Series{}).Where("id = ?", series.ID).Updates(map[string]any{
		"title":    series.Title,
		"position": series.Position,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteSeries(database *gorm.DB, seriesID uint) error {
	result := database.Delete(&Series{}, seriesID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
