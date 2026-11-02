// Package projectview describes one validated project without running its code.
package projectview

import (
	"fmt"
	"slices"

	"github.com/wavykit/wsup/internal/guidance"
	"github.com/wavykit/wsup/internal/registry"
)

type Workspace struct {
	Name string `json:"name"`
}

type Project struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Repository string `json:"repository"`
	Purpose    string `json:"purpose"`
	Owner      string `json:"owner"`
}

type CheckPolicy struct {
	Mode             string `json:"mode"`
	Enabled          bool   `json:"enabled"`
	Reason           string `json:"reason"`
	TimeoutSeconds   *int   `json:"timeout_seconds"`
	FindingExitCodes []int  `json:"finding_exit_codes"`
}

type Task struct {
	Name        string       `json:"name"`
	Command     []string     `json:"command"`
	Cwd         string       `json:"cwd"`
	Check       bool         `json:"check"`
	CheckPolicy *CheckPolicy `json:"check_policy"`
}

type Result struct {
	Workspace    Workspace             `json:"workspace"`
	Project      Project               `json:"project"`
	Tasks        []Task                `json:"tasks"`
	Dependencies []string              `json:"dependencies"`
	Dependents   []string              `json:"dependents"`
	Instructions []string              `json:"instructions"`
	Skills       []guidance.OwnedSkill `json:"skills"`
}

// Inspect uses the registry's validation and ordering, not a second parser or
// a task availability probe. The caller separately validates CLI configuration.
func Inspect(root, id string) (Result, error) {
	data, err := registry.Load(root)
	if err != nil {
		return Result{}, err
	}
	index := slices.IndexFunc(data.Projects, func(project registry.Project) bool { return project.ID == id })
	if index < 0 {
		return Result{}, fmt.Errorf("Unknown project: %s", id)
	}
	project := data.Projects[index]
	discovered, err := guidance.InspectProject(root, project.Path)
	if err != nil {
		return Result{}, err
	}
	entry := project.Entry
	result := Result{
		Workspace: Workspace{Name: data.Name},
		Project: Project{ID: entry.ID, Path: entry.Path, Kind: entry.Kind,
			Repository: entry.Repository, Purpose: entry.Purpose, Owner: entry.Owner},
		Tasks: []Task{}, Dependencies: append([]string{}, project.Dependencies...), Dependents: []string{},
		Instructions: discovered.Instructions, Skills: discovered.Skills,
	}
	for _, task := range project.Tasks {
		item := Task{Name: task.Name, Command: append([]string{}, task.Check.Command...), Cwd: task.Cwd, Check: task.IsCheck}
		if task.IsCheck {
			policy := task.Check
			item.CheckPolicy = &CheckPolicy{Mode: policy.Mode, Enabled: policy.Enabled, Reason: policy.Reason,
				TimeoutSeconds: policy.TimeoutSeconds, FindingExitCodes: append([]int{}, policy.FindingExitCodes...)}
		}
		result.Tasks = append(result.Tasks, item)
	}
	for _, other := range data.Projects {
		if slices.Contains(other.Dependencies, id) {
			result.Dependents = append(result.Dependents, other.ID)
		}
	}
	return result, nil
}
