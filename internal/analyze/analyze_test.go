package analyze

import (
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
)

func TestFile(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		targets []ir.Cloud
		want    []string
		report  bool
	}{
		{
			name: "front-end errors hide the matrix",
			src:  "use aws\nbucket b is shiny\n",
			want: []string{"unknown bucket trait `shiny`"},
		},
		{
			name:   "capability findings",
			src:    "use aws\nuse gcp\nqueue q is fifo\n",
			want:   []string{"on gcp: Pub/Sub has no FIFO queues; this uses an ordered, exactly-once subscription, which limits throughput"},
			report: true,
		},
		{
			name: "lowering errors hide the matrix",
			src:  "use aws\nbucket b\n  on aws\n    force_destroyy true\n",
			want: []string{"aws_s3_bucket has no setting `force_destroyy` that `on aws` can change"},
		},
		{
			name:    "targets replace the use lines",
			src:     "use aws\nqueue q is fifo\n",
			targets: []ir.Cloud{ir.GCP},
			want:    []string{"on gcp: Pub/Sub has no FIFO queues; this uses an ordered, exactly-once subscription, which limits throughput"},
			report:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := File(diag.NewFile("t.clowd", tt.src), tt.targets)
			var got []string
			for _, d := range a.All {
				got = append(got, d.Message)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("diagnostics = %q, want %q", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("diagnostic %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
			if (a.Report != nil) != tt.report {
				t.Errorf("Report = %v, want present: %v", a.Report, tt.report)
			}
		})
	}
}
