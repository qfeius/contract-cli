package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type cooperationSearchInput struct {
	PageSize       *int     `json:"page_size,omitempty"`
	PageToken      string   `json:"page_token,omitempty"`
	Keyword        string   `json:"keyword,omitempty"`
	SearchTab      int      `json:"search_tab_enum_code"`
	DisplayTab     int      `json:"cooperation_display_tab_code"`
	StatusCodes    []int    `json:"cooperation_query_status_codes,omitempty"`
	CategoryIDs    []string `json:"contract_category_ids,omitempty"`
	CreateTime     []string `json:"search_cooperation_by_create_time,omitempty"`
	ConfirmTime    []string `json:"search_cooperation_by_last_confirm_time,omitempty"`
	UploadTime     []string `json:"search_cooperation_by_latest_version_upload_time,omitempty"`
	CompleteTime   []string `json:"search_cooperation_by_cooperation_complete_time,omitempty"`
	CreateUserID   string   `json:"create_user_id,omitempty"`
	Departments    []string `json:"department_collection,omitempty"`
	ConfirmedRoles []string `json:"confirmed_role_codes,omitempty"`
	Order          string   `json:"order"`
}

func cooperationSearchBody(ctx context.Context, options commandOptions) ([]byte, error) {
	if options.userID != "" || options.userIDType != "" || options.departmentIDType != "" {
		return nil, fmt.Errorf("user cooperation search fixes user_id_type=user_id and department_id_type=open_department_id; caller overrides are not supported")
	}

	object, err := resolveJSONObjectBody(ctx, options, true)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	pageSize := 10
	input := cooperationSearchInput{PageSize: &pageSize, Order: "DESC"}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("invalid user cooperation search: %w", err)
	}
	if err := validateCooperationSearch(input); err != nil {
		return nil, err
	}
	return json.Marshal(input)
}

func validateCooperationSearch(input cooperationSearchInput) error {
	if input.PageSize == nil || *input.PageSize < 1 || *input.PageSize > 20 {
		return fmt.Errorf("page_size must be an integer from 1 to 20")
	}
	if input.SearchTab < 0 || input.SearchTab > 1 {
		return fmt.Errorf("search_tab_enum_code must be 0 or 1")
	}
	if input.DisplayTab < 0 || input.DisplayTab > 5 {
		return fmt.Errorf("cooperation_display_tab_code must be between 0 and 5")
	}
	if input.Order != "ASC" && input.Order != "DESC" {
		return fmt.Errorf("order must be ASC or DESC")
	}
	for _, dateRange := range []struct {
		name  string
		dates []string
	}{
		{"search_cooperation_by_create_time", input.CreateTime},
		{"search_cooperation_by_last_confirm_time", input.ConfirmTime},
		{"search_cooperation_by_latest_version_upload_time", input.UploadTime},
		{"search_cooperation_by_cooperation_complete_time", input.CompleteTime},
	} {
		if err := validateCooperationDateRange(dateRange.name, dateRange.dates); err != nil {
			return err
		}
	}
	return nil
}

func validateCooperationDateRange(name string, dates []string) error {
	if len(dates) == 0 {
		return nil
	}
	if len(dates) != 2 {
		return fmt.Errorf("%s must contain start and end dates", name)
	}
	for _, value := range dates {
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("%s must use valid YYYY-MM-DD dates", name)
		}
	}
	if dates[0] > dates[1] {
		return fmt.Errorf("%s start date must not exceed end date", name)
	}
	return nil
}
