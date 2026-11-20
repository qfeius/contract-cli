package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode"
)

type projectSearchResult struct {
	ProjectID string `json:"project_id"`
	BasicInfo struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	} `json:"basic_info"`
	Department struct {
		DisplayName string `json:"display_name"`
	} `json:"department"`
	FieldValues []struct {
		FieldKey     string          `json:"field_key"`
		DisplayValue json.RawMessage `json:"display_value"`
	} `json:"field_values"`
}

// writeContractProjectTable reports whether the successful response is a project page.
func writeContractProjectTable(stdout io.Writer, body []byte) (bool, error) {
	var envelope struct {
		Data struct {
			Content       []projectSearchResult `json:"content"`
			PageIndex     *int64                `json:"page_index"`
			TotalElements *int64                `json:"total_elements"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Data.Content == nil {
		return false, nil
	}

	page := envelope.Data
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if len(page.Content) == 0 {
		if _, err := fmt.Fprintln(table, "本页无项目"); err != nil {
			return true, err
		}
	} else {
		if _, err := fmt.Fprintln(table, "项目名称\t状态\t所属部门\t项目周期\t项目 ID"); err != nil {
			return true, err
		}
		for _, project := range page.Content {
			cells := []string{
				projectTableCell(project.BasicInfo.Name),
				projectTableCell(project.statusLabel()),
				projectTableCell(project.Department.DisplayName),
				project.periodLabel(),
				projectTableCell(project.ProjectID),
			}
			if _, err := fmt.Fprintln(table, strings.Join(cells, "\t")); err != nil {
				return true, err
			}
		}
	}
	if err := table.Flush(); err != nil {
		return true, err
	}

	var summary []string
	if page.PageIndex != nil {
		summary = append(summary, fmt.Sprintf("第 %d 页", *page.PageIndex+1))
	}
	if page.TotalElements != nil {
		summary = append(summary, fmt.Sprintf("共 %d 个项目", *page.TotalElements))
	}
	if len(summary) > 0 {
		_, err := fmt.Fprintln(stdout, strings.Join(summary, "，"))
		return true, err
	}
	return true, nil
}

func (project projectSearchResult) statusLabel() string {
	for _, field := range project.FieldValues {
		if field.FieldKey == "project_status" {
			var label string
			if json.Unmarshal(field.DisplayValue, &label) == nil {
				return label
			}
			break
		}
	}
	return ""
}

func (project projectSearchResult) periodLabel() string {
	start, end := project.BasicInfo.StartDate, project.BasicInfo.EndDate
	if start == "" && end == "" {
		return "—"
	}
	return projectTableCell(start) + " 至 " + projectTableCell(end)
}

func projectTableCell(value string) string {
	// Keep server-supplied text on one line and out of terminal control sequences.
	value = strings.TrimSpace(strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value))
	if value == "" {
		return "—"
	}
	return value
}
