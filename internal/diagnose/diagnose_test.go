package diagnose

import (
	"bytes"
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(name, ns string, phase corev1.PodPhase, restarts int32, reason string) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: corev1.PodStatus{
			Phase: phase,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: restarts,
				},
			},
		},
	}
	if reason != "" {
		p.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: reason},
		}
	}
	return p
}

func TestDiagnose_TableDriven(t *testing.T) {
	cases := []struct {
		name      string
		pods      []*corev1.Pod
		threshold int
		wantLines []string
		dontWant  []string
	}{
		{
			name:      "no issues",
			pods:      []*corev1.Pod{pod("ok-pod", "default", corev1.PodRunning, 0, "")},
			threshold: 5,
			wantLines: []string{"No issues found"},
		},
		{
			name:      "high restart count flagged",
			pods:      []*corev1.Pod{pod("crashy", "default", corev1.PodRunning, 8, "OOMKilled")},
			threshold: 5,
			wantLines: []string{"crashy", "8", "OOMKilled"},
		},
		{
			name:      "below threshold not flagged",
			pods:      []*corev1.Pod{pod("stable", "default", corev1.PodRunning, 3, "")},
			threshold: 5,
			wantLines: []string{"No issues found"},
			dontWant:  []string{"stable"},
		},
		{
			name: "pending pod flagged",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "stuck", Namespace: "default"},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
			threshold: 5,
			wantLines: []string{"PENDING", "stuck"},
		},
		{
			name: "multiple issues",
			pods: []*corev1.Pod{
				pod("crashy", "default", corev1.PodRunning, 10, "Error"),
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pending-pod", Namespace: "kube-system"},
					Status:     corev1.PodStatus{Phase: corev1.PodPending},
				},
			},
			threshold: 5,
			wantLines: []string{"crashy", "10", "Error", "pending-pod"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			for _, p := range tc.pods {
				if _, err := client.CoreV1().Pods(p.Namespace).Create(
					context.Background(), p, metav1.CreateOptions{},
				); err != nil {
					t.Fatalf("create pod: %v", err)
				}
			}

			var buf bytes.Buffer
			err := Run(context.Background(), client, Options{
				RestartThreshold: tc.threshold,
				Namespace:        "",
				Out:              &buf,
			})
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			out := buf.String()
			for _, want := range tc.wantLines {
				if !strings.Contains(out, want) {
					t.Errorf("expected output to contain %q\ngot:\n%s", want, out)
				}
			}
			for _, dont := range tc.dontWant {
				if strings.Contains(out, dont) {
					t.Errorf("expected output NOT to contain %q\ngot:\n%s", dont, out)
				}
			}
		})
	}
}
