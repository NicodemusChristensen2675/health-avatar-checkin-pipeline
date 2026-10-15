package avatars

import (
	"context"
	"fmt"
)

type Processor interface {
	Upload(context.Context, []byte, string, string) (string, error)
	SmartCrop(context.Context, string, string) (string, error)
	Optimize(context.Context, string, string) (string, error)
	SetUserAvatar(context.Context, string, string, string) error
}

type AppointmentStatus string

const (
	AppointmentScheduled AppointmentStatus = "scheduled"
	AppointmentConfirmed AppointmentStatus = "confirmed"
	AppointmentCompleted AppointmentStatus = "completed"
	AppointmentCancelled AppointmentStatus = "cancelled"
)

type Input struct {
	UserID            string
	Filename          string
	File              []byte
	RequestID         string
	AppointmentStatus AppointmentStatus
}

type Result struct {
	ImageID      string `json:"image_id"`
	Notification string `json:"notification"`
}

type Pipeline struct {
	processor Processor
}

func New(processor Processor) *Pipeline {
	return &Pipeline{processor: processor}
}

func (p *Pipeline) Run(ctx context.Context, in Input) (Result, error) {
	uploaded, err := p.processor.Upload(ctx, in.File, in.Filename, in.RequestID)
	if err != nil {
		return Result{}, fmt.Errorf("upload avatar: %w", err)
	}
	cropped, err := p.processor.SmartCrop(ctx, uploaded, in.RequestID)
	if err != nil {
		return Result{}, fmt.Errorf("crop avatar: %w", err)
	}
	optimized, err := p.processor.Optimize(ctx, cropped, in.RequestID)
	if err != nil {
		return Result{}, fmt.Errorf("optimize avatar: %w", err)
	}
	if err := p.processor.SetUserAvatar(ctx, in.UserID, optimized, in.RequestID); err != nil {
		return Result{}, fmt.Errorf("update user avatar: %w", err)
	}
	return Result{ImageID: optimized, Notification: notificationFor(in.AppointmentStatus)}, nil
}

func notificationFor(status AppointmentStatus) string {
	switch status {
	case AppointmentScheduled, AppointmentConfirmed:
		return "Profile photo updated. Your care team will see it at check-in."
	default:
		return "Profile photo updated."
	}
}
