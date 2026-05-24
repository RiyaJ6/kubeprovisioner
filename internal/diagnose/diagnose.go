// Package diagnose surfaces pods with high restart counts, OOMKills, or pending state.
package diagnose

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Options configures the diagnose run.
type Options struct {
	RestartThreshold int
	Namespace        string
	Out              io.Writer
}

// PodIssue describes a pod that has been flagged.
type PodIssue struct {
	Namespace           string
	Name                string
	Phase               corev1.PodPhase
	Restarts            int32
	LastTerminationReason string
	Container           string
	PendingSince        string
}

// Run queries the cluster and prints flagged pods to opts.Out.
func Run(ctx context.Context, client kubernetes.Interface, opts Options) error {
	pods, err := client.CoreV1().Pods(opts.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	var highRestart []PodIssue
	var pending []PodIssue

	for _, pod := range pods.Items {
		// check container statuses for restarts and OOMKills
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.RestartCount >= int32(opts.RestartThreshold) {
				issue := PodIssue{
					Namespace: pod.Namespace,
					Name:      pod.Name,
					Phase:     pod.Status.Phase,
					Restarts:  cs.RestartCount,
					Container: cs.Name,
				}
				if cs.LastTerminationState.Terminated != nil {
					issue.LastTerminationReason = cs.LastTerminationState.Terminated.Reason
				}
				highRestart = append(highRestart, issue)
			}
		}

		// check for pods stuck in Pending
		if pod.Status.Phase == corev1.PodPending {
			age := "unknown"
			if !pod.CreationTimestamp.IsZero() {
				age = metav1.Now().Sub(pod.CreationTimestamp.Time).Round(1e9).String()
			}
			pending = append(pending, PodIssue{
				Namespace:    pod.Namespace,
				Name:         pod.Name,
				Phase:        pod.Status.Phase,
				PendingSince: age,
			})
		}
	}

	w := tabwriter.NewWriter(opts.Out, 0, 0, 3, ' ', 0)

	if len(highRestart) == 0 && len(pending) == 0 {
		fmt.Fprintln(opts.Out, "No issues found.")
		return nil
	}

	if len(highRestart) > 0 {
		fmt.Fprintf(opts.Out, "PODS WITH HIGH RESTART COUNTS (threshold: %d)\n", opts.RestartThreshold)
		fmt.Fprintln(w, "  NAMESPACE\tNAME\tCONTAINER\tRESTARTS\tLAST_REASON")
		for _, p := range highRestart {
			reason := p.LastTerminationReason
			if reason == "" {
				reason = "—"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%d\t%s\n", p.Namespace, p.Name, p.Container, p.Restarts, reason)
		}
		w.Flush()
	}

	if len(pending) > 0 {
		fmt.Fprintln(opts.Out, "\nPENDING PODS")
		fmt.Fprintln(w, "  NAMESPACE\tNAME\tPENDING_SINCE")
		for _, p := range pending {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", p.Namespace, p.Name, p.PendingSince)
		}
		w.Flush()
	}

	return nil
}
