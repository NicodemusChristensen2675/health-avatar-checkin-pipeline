package avatars

import (
	"context"
	"testing"
)

type fakeProcessor struct {
	steps []string
}

func (f *fakeProcessor) Upload(context.Context, []byte, string, string) (string, error) {
	f.steps = append(f.steps, "upload")
	return "uploaded", nil
}

func (f *fakeProcessor) SmartCrop(_ context.Context, imageID, _ string) (string, error) {
	f.steps = append(f.steps, "crop:"+imageID)
	return "cropped", nil
}

func (f *fakeProcessor) Optimize(_ context.Context, imageID, _ string) (string, error) {
	f.steps = append(f.steps, "optimize:"+imageID)
	return "optimized", nil
}

func (f *fakeProcessor) SetUserAvatar(_ context.Context, userID, imageID, _ string) error {
	f.steps = append(f.steps, "user:"+userID+":"+imageID)
	return nil
}

func TestPipelineAppointmentNotification(t *testing.T) {
	tests := []struct {
		name   string
		status AppointmentStatus
		want   string
	}{
		{"scheduled check-in", AppointmentScheduled, "Profile photo updated. Your care team will see it at check-in."},
		{"confirmed check-in", AppointmentConfirmed, "Profile photo updated. Your care team will see it at check-in."},
		{"completed appointment", AppointmentCompleted, "Profile photo updated."},
		{"cancelled appointment", AppointmentCancelled, "Profile photo updated."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &fakeProcessor{}
			result, err := New(processor).Run(context.Background(), Input{
				UserID: "patient-42", Filename: "avatar.jpg", File: []byte("photo"),
				RequestID: "request-7", AppointmentStatus: tt.status,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Notification != tt.want {
				t.Fatalf("notification = %q, want %q", result.Notification, tt.want)
			}
			if result.ImageID != "optimized" {
				t.Fatalf("image ID = %q", result.ImageID)
			}
			wantSteps := []string{"upload", "crop:uploaded", "optimize:cropped", "user:patient-42:optimized"}
			for i := range wantSteps {
				if processor.steps[i] != wantSteps[i] {
					t.Fatalf("step %d = %q, want %q", i, processor.steps[i], wantSteps[i])
				}
			}
		})
	}
}
