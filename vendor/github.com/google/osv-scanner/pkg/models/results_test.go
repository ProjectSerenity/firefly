package models

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFlatten(t *testing.T) {
	t.Parallel()
	// Test case 1: When there are no vulnerabilities
	vulns := VulnerabilityResults{Results: []PackageSource{}}
	expectedFlattened := []VulnerabilityFlattened{}
	flattened := vulns.Flatten()
	if diff := cmp.Diff(flattened, expectedFlattened); diff != "" {
		t.Errorf("Flatten() returned unexpected result (-got +want):\n%s", diff)
	}

	// Test case 2: When there are vulnerabilities
	group := GroupInfo{IDs: []string{"CVE-2021-1234"}}
	pkg := PackageVulns{
		Package: PackageInfo{Name: "package"},
		Groups:  []GroupInfo{group},
		Vulnerabilities: []Vulnerability{
			{
				ID: "CVE-2021-1234",
				Severity: []Severity{
					{
						Type:  SeverityType("high"),
						Score: "1",
					},
				},
			},
		},
	}
	source := PackageSource{Source: SourceInfo{Path: "package"}, Packages: []PackageVulns{pkg}}
	vulns = VulnerabilityResults{Results: []PackageSource{source}}
	expectedFlattened = []VulnerabilityFlattened{
		{
			Source:        source.Source,
			Package:       pkg.Package,
			Vulnerability: pkg.Vulnerabilities[0],
			GroupInfo:     group,
		},
	}
	flattened = vulns.Flatten()
	if diff := cmp.Diff(flattened, expectedFlattened); diff != "" {
		t.Errorf("Flatten() returned unexpected result (-got +want):\n%s", diff)
	}
}
