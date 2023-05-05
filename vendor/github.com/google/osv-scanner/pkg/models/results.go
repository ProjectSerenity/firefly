package models

import (
	"golang.org/x/exp/slices"
)

// Combined vulnerabilities found for the scanned packages
type VulnerabilityResults struct {
	Results []PackageSource `json:"results"`
}

// Flatten the grouped/nested vulnerability results into one flat array.
func (vulns *VulnerabilityResults) Flatten() []VulnerabilityFlattened {
	results := []VulnerabilityFlattened{}
	for _, res := range vulns.Results {
		for _, pkg := range res.Packages {
			for _, v := range pkg.Vulnerabilities {
				// groupIdx should never be -1 since vulnerabilities should always be in one group
				groupIdx := slices.IndexFunc(pkg.Groups, func(g GroupInfo) bool { return slices.Contains(g.IDs, v.ID) })
				results = append(results, VulnerabilityFlattened{
					Source:        res.Source,
					Package:       pkg.Package,
					Vulnerability: v,
					GroupInfo:     pkg.Groups[groupIdx],
				})
			}
		}
	}

	return results
}

// Flattened Vulnerability Information.
type VulnerabilityFlattened struct {
	Source        SourceInfo
	Package       PackageInfo
	Vulnerability Vulnerability
	GroupInfo     GroupInfo
}

type SourceInfo struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (s SourceInfo) String() string {
	return s.Type + ":" + s.Path
}

// Vulnerabilities grouped by sources
type PackageSource struct {
	Source   SourceInfo     `json:"source"`
	Packages []PackageVulns `json:"packages"`
}

// Vulnerabilities grouped by package
type PackageVulns struct {
	Package         PackageInfo     `json:"package"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
	Groups          []GroupInfo     `json:"groups"`
}

type GroupInfo struct {
	IDs []string `json:"ids"`
	// Map of Vulnerability IDs to AnalysisInfo
	ExperimentalAnalysis map[string]AnalysisInfo `json:"experimentalAnalysis,omitempty"`
}

// IsCalled returns true if any analysis performed determines that the vulnerability is being called
// Also returns true if no analysis is performed
func (groupInfo *GroupInfo) IsCalled() bool {
	if len(groupInfo.ExperimentalAnalysis) == 0 {
		return true
	}

	for _, analysis := range groupInfo.ExperimentalAnalysis {
		if analysis.Called {
			return true
		}
	}

	return false
}

type AnalysisInfo struct {
	Called bool `json:"called"`
}

// Specific package information
type PackageInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Ecosystem string `json:"ecosystem"`
}
