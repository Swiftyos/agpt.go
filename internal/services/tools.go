package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agpt-go/chatbot-api/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ToolService handles AI tool execution
type ToolService struct {
	queries *database.Queries
}

// NewToolService creates a new tool service
func NewToolService(queries *database.Queries) *ToolService {
	return &ToolService{
		queries: queries,
	}
}

// AddUnderstandingInput represents the input for the add_understanding tool
type AddUnderstandingInput struct {
	UserName           string   `json:"user_name,omitempty"`
	JobTitle           string   `json:"job_title,omitempty"`
	BusinessName       string   `json:"business_name,omitempty"`
	Industry           string   `json:"industry,omitempty"`
	BusinessSize       string   `json:"business_size,omitempty"`
	UserRole           string   `json:"user_role,omitempty"`
	KeyWorkflows       []string `json:"key_workflows,omitempty"`
	DailyActivities    []string `json:"daily_activities,omitempty"`
	PainPoints         []string `json:"pain_points,omitempty"`
	Bottlenecks        []string `json:"bottlenecks,omitempty"`
	ManualTasks        []string `json:"manual_tasks,omitempty"`
	AutomationGoals    []string `json:"automation_goals,omitempty"`
	CurrentSoftware    []string `json:"current_software,omitempty"`
	ExistingAutomation []string `json:"existing_automation,omitempty"`
	AdditionalNotes    string   `json:"additional_notes,omitempty"`
}

// UnderstandingStatus represents the current understanding status
type UnderstandingStatus struct {
	UserName           bool `json:"user_name"`
	JobTitle           bool `json:"job_title"`
	BusinessName       bool `json:"business_name"`
	Industry           bool `json:"industry"`
	BusinessSize       bool `json:"business_size"`
	UserRole           bool `json:"user_role"`
	KeyWorkflowsCount  int  `json:"key_workflows_count"`
	DailyActivitiesCount int `json:"daily_activities_count"`
	PainPointsCount    int  `json:"pain_points_count"`
	BottlenecksCount   int  `json:"bottlenecks_count"`
	ManualTasksCount   int  `json:"manual_tasks_count"`
	AutomationGoalsCount int `json:"automation_goals_count"`
	CurrentSoftwareCount int `json:"current_software_count"`
	ExistingAutomationCount int `json:"existing_automation_count"`
	AdditionalNotes    bool `json:"additional_notes"`
}

// AddUnderstandingResponse represents the response from the add_understanding tool
type AddUnderstandingResponse struct {
	Message       string              `json:"message"`
	UpdatedFields []string            `json:"updated_fields"`
	Status        UnderstandingStatus `json:"status"`
	NextSteps     string              `json:"next_steps"`
}

// BusinessContext represents the full business context for a user
type BusinessContext struct {
	UserName           string   `json:"user_name,omitempty"`
	JobTitle           string   `json:"job_title,omitempty"`
	BusinessName       string   `json:"business_name,omitempty"`
	Industry           string   `json:"industry,omitempty"`
	BusinessSize       string   `json:"business_size,omitempty"`
	UserRole           string   `json:"user_role,omitempty"`
	KeyWorkflows       []string `json:"key_workflows,omitempty"`
	DailyActivities    []string `json:"daily_activities,omitempty"`
	PainPoints         []string `json:"pain_points,omitempty"`
	Bottlenecks        []string `json:"bottlenecks,omitempty"`
	ManualTasks        []string `json:"manual_tasks,omitempty"`
	AutomationGoals    []string `json:"automation_goals,omitempty"`
	CurrentSoftware    []string `json:"current_software,omitempty"`
	ExistingAutomation []string `json:"existing_automation,omitempty"`
	AdditionalNotes    string   `json:"additional_notes,omitempty"`
}

// GetAddUnderstandingToolDefinition returns the tool definition for add_understanding
func GetAddUnderstandingToolDefinition() ToolDefinition {
	return ToolDefinition{
		Name: "add_understanding",
		Description: `Capture and store information about the user's business context,
workflows, pain points, and automation goals. Call this tool whenever the user
shares information about their business. Each call incrementally adds to the
existing understanding - you don't need to provide all fields at once.

Use this to build a comprehensive profile that helps recommend better agents
and automations for the user's specific needs.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user_name": map[string]interface{}{
					"type":        "string",
					"description": "The user's name",
				},
				"job_title": map[string]interface{}{
					"type":        "string",
					"description": "The user's job title (e.g., 'Marketing Manager', 'CEO', 'Software Engineer')",
				},
				"business_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the user's business or organization",
				},
				"industry": map[string]interface{}{
					"type":        "string",
					"description": "Industry or sector (e.g., 'e-commerce', 'healthcare', 'finance')",
				},
				"business_size": map[string]interface{}{
					"type":        "string",
					"description": "Company size: '1-10', '11-50', '51-200', '201-1000', or '1000+'",
				},
				"user_role": map[string]interface{}{
					"type":        "string",
					"description": "User's role in organization context (e.g., 'decision maker', 'implementer', 'end user')",
				},
				"key_workflows": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Key business workflows (e.g., 'lead qualification', 'content publishing')",
				},
				"daily_activities": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Regular daily activities the user performs",
				},
				"pain_points": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Current pain points or challenges",
				},
				"bottlenecks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Process bottlenecks slowing things down",
				},
				"manual_tasks": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Manual or repetitive tasks that could be automated",
				},
				"automation_goals": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Desired automation outcomes or goals",
				},
				"current_software": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Software and tools currently in use",
				},
				"existing_automation": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Any existing automations or integrations",
				},
				"additional_notes": map[string]interface{}{
					"type":        "string",
					"description": "Any other relevant context or notes",
				},
			},
			"required": []string{},
		},
	}
}

// GetGenerateBusinessReportToolDefinition returns the tool definition for generate_business_report
func GetGenerateBusinessReportToolDefinition() ToolDefinition {
	return ToolDefinition{
		Name: "generate_business_report",
		Description: `Generate a comprehensive AI integration report for the user's business.
This tool analyzes all gathered business understanding and creates a personalized
report with recommendations for AI agents and automations that could benefit their
specific workflows and pain points.

Only call this tool when you have gathered sufficient understanding about the user's
business context, workflows, and goals.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"report_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"executive_summary", "detailed", "quick_wins"},
					"description": "Type of report to generate: 'executive_summary' for high-level overview, 'detailed' for comprehensive analysis, 'quick_wins' for immediate action items",
				},
				"focus_areas": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Specific areas to focus the report on (e.g., 'automation', 'cost reduction', 'efficiency')",
				},
			},
			"required": []string{"report_type"},
		},
	}
}

// GetAllToolDefinitions returns all available tool definitions
func GetAllToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		GetAddUnderstandingToolDefinition(),
		GetGenerateBusinessReportToolDefinition(),
	}
}

// ExecuteAddUnderstanding executes the add_understanding tool
func (s *ToolService) ExecuteAddUnderstanding(ctx context.Context, userID uuid.UUID, input AddUnderstandingInput) (*AddUnderstandingResponse, error) {
	// Check if any data was provided
	if !hasAnyInput(input) {
		return nil, fmt.Errorf("please provide at least one field to update")
	}

	// Get existing understanding or create new
	existing, err := s.queries.GetBusinessUnderstanding(ctx, userID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to get existing understanding: %w", err)
	}

	// Merge arrays - deduplicate and append
	keyWorkflows := mergeStringArrays(jsonToStringArray(existing.KeyWorkflows), input.KeyWorkflows)
	dailyActivities := mergeStringArrays(jsonToStringArray(existing.DailyActivities), input.DailyActivities)
	painPoints := mergeStringArrays(jsonToStringArray(existing.PainPoints), input.PainPoints)
	bottlenecks := mergeStringArrays(jsonToStringArray(existing.Bottlenecks), input.Bottlenecks)
	manualTasks := mergeStringArrays(jsonToStringArray(existing.ManualTasks), input.ManualTasks)
	automationGoals := mergeStringArrays(jsonToStringArray(existing.AutomationGoals), input.AutomationGoals)
	currentSoftware := mergeStringArrays(jsonToStringArray(existing.CurrentSoftware), input.CurrentSoftware)
	existingAutomation := mergeStringArrays(jsonToStringArray(existing.ExistingAutomation), input.ExistingAutomation)

	// Upsert the understanding
	params := database.UpsertBusinessUnderstandingParams{
		UserID:             userID,
		UserName:           stringPtrOrNil(input.UserName),
		JobTitle:           stringPtrOrNil(input.JobTitle),
		BusinessName:       stringPtrOrNil(input.BusinessName),
		Industry:           stringPtrOrNil(input.Industry),
		BusinessSize:       stringPtrOrNil(input.BusinessSize),
		UserRole:           stringPtrOrNil(input.UserRole),
		KeyWorkflows:       stringArrayToJSON(keyWorkflows),
		DailyActivities:    stringArrayToJSON(dailyActivities),
		PainPoints:         stringArrayToJSON(painPoints),
		Bottlenecks:        stringArrayToJSON(bottlenecks),
		ManualTasks:        stringArrayToJSON(manualTasks),
		AutomationGoals:    stringArrayToJSON(automationGoals),
		CurrentSoftware:    stringArrayToJSON(currentSoftware),
		ExistingAutomation: stringArrayToJSON(existingAutomation),
		AdditionalNotes:    stringPtrOrNil(input.AdditionalNotes),
	}

	understanding, err := s.queries.UpsertBusinessUnderstanding(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update understanding: %w", err)
	}

	// Track which fields were updated
	updatedFields := getUpdatedFields(input)

	// Build status
	status := buildUnderstandingStatus(understanding)

	// Generate next steps message
	nextSteps := generateNextStepsMessage(status)

	return &AddUnderstandingResponse{
		Message:       fmt.Sprintf("Updated understanding with: %s. I now have a better picture of your business context.", strings.Join(updatedFields, ", ")),
		UpdatedFields: updatedFields,
		Status:        status,
		NextSteps:     nextSteps,
	}, nil
}

// GenerateBusinessReportInput represents the input for the generate_business_report tool
type GenerateBusinessReportInput struct {
	ReportType  string   `json:"report_type"`
	FocusAreas  []string `json:"focus_areas,omitempty"`
}

// BusinessReportResponse represents the response from the generate_business_report tool
type BusinessReportResponse struct {
	Message         string          `json:"message"`
	BusinessContext *BusinessContext `json:"business_context"`
	ReportType      string          `json:"report_type"`
	Status          string          `json:"status"`
}

// ExecuteGenerateBusinessReport executes the generate_business_report tool (stub)
func (s *ToolService) ExecuteGenerateBusinessReport(ctx context.Context, userID uuid.UUID, input GenerateBusinessReportInput) (*BusinessReportResponse, error) {
	// Get the current understanding
	understanding, err := s.queries.GetBusinessUnderstanding(ctx, userID)
	if err == pgx.ErrNoRows {
		return &BusinessReportResponse{
			Message:    "I don't have enough information about your business yet. Let's start by learning more about your company, your role, and what challenges you're facing.",
			Status:     "insufficient_data",
			ReportType: input.ReportType,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get business understanding: %w", err)
	}

	// Build business context
	businessContext := &BusinessContext{
		UserName:           derefStringPtr(understanding.UserName),
		JobTitle:           derefStringPtr(understanding.JobTitle),
		BusinessName:       derefStringPtr(understanding.BusinessName),
		Industry:           derefStringPtr(understanding.Industry),
		BusinessSize:       derefStringPtr(understanding.BusinessSize),
		UserRole:           derefStringPtr(understanding.UserRole),
		KeyWorkflows:       jsonToStringArray(understanding.KeyWorkflows),
		DailyActivities:    jsonToStringArray(understanding.DailyActivities),
		PainPoints:         jsonToStringArray(understanding.PainPoints),
		Bottlenecks:        jsonToStringArray(understanding.Bottlenecks),
		ManualTasks:        jsonToStringArray(understanding.ManualTasks),
		AutomationGoals:    jsonToStringArray(understanding.AutomationGoals),
		CurrentSoftware:    jsonToStringArray(understanding.CurrentSoftware),
		ExistingAutomation: jsonToStringArray(understanding.ExistingAutomation),
		AdditionalNotes:    derefStringPtr(understanding.AdditionalNotes),
	}

	// Check if we have enough data for a meaningful report
	status := buildUnderstandingStatus(understanding)
	if !hasMinimumDataForReport(status) {
		return &BusinessReportResponse{
			Message:         generateDataGapsMessage(status),
			BusinessContext: businessContext,
			Status:          "needs_more_data",
			ReportType:      input.ReportType,
		}, nil
	}

	// For now, return a stub response indicating report generation is pending
	return &BusinessReportResponse{
		Message: fmt.Sprintf("Business AI Report (%s) generation is ready. Based on the understanding gathered, I can provide recommendations for AI integration. Full report generation will be implemented in a future update.",
			input.ReportType),
		BusinessContext: businessContext,
		Status:          "ready_for_report",
		ReportType:      input.ReportType,
	}, nil
}

// GetBusinessContext returns the current business context for a user
func (s *ToolService) GetBusinessContext(ctx context.Context, userID uuid.UUID) (*BusinessContext, error) {
	understanding, err := s.queries.GetBusinessUnderstanding(ctx, userID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get business understanding: %w", err)
	}

	return &BusinessContext{
		UserName:           derefStringPtr(understanding.UserName),
		JobTitle:           derefStringPtr(understanding.JobTitle),
		BusinessName:       derefStringPtr(understanding.BusinessName),
		Industry:           derefStringPtr(understanding.Industry),
		BusinessSize:       derefStringPtr(understanding.BusinessSize),
		UserRole:           derefStringPtr(understanding.UserRole),
		KeyWorkflows:       jsonToStringArray(understanding.KeyWorkflows),
		DailyActivities:    jsonToStringArray(understanding.DailyActivities),
		PainPoints:         jsonToStringArray(understanding.PainPoints),
		Bottlenecks:        jsonToStringArray(understanding.Bottlenecks),
		ManualTasks:        jsonToStringArray(understanding.ManualTasks),
		AutomationGoals:    jsonToStringArray(understanding.AutomationGoals),
		CurrentSoftware:    jsonToStringArray(understanding.CurrentSoftware),
		ExistingAutomation: jsonToStringArray(understanding.ExistingAutomation),
		AdditionalNotes:    derefStringPtr(understanding.AdditionalNotes),
	}, nil
}

// BuildSystemPromptContext generates a context string to include in the system prompt
func (s *ToolService) BuildSystemPromptContext(ctx context.Context, userID uuid.UUID) (string, error) {
	businessContext, err := s.GetBusinessContext(ctx, userID)
	if err != nil {
		return "", err
	}
	if businessContext == nil {
		return "", nil
	}

	return buildBusinessContextPrompt(businessContext), nil
}

// Helper functions

func hasAnyInput(input AddUnderstandingInput) bool {
	return input.UserName != "" ||
		input.JobTitle != "" ||
		input.BusinessName != "" ||
		input.Industry != "" ||
		input.BusinessSize != "" ||
		input.UserRole != "" ||
		len(input.KeyWorkflows) > 0 ||
		len(input.DailyActivities) > 0 ||
		len(input.PainPoints) > 0 ||
		len(input.Bottlenecks) > 0 ||
		len(input.ManualTasks) > 0 ||
		len(input.AutomationGoals) > 0 ||
		len(input.CurrentSoftware) > 0 ||
		len(input.ExistingAutomation) > 0 ||
		input.AdditionalNotes != ""
}

func getUpdatedFields(input AddUnderstandingInput) []string {
	var fields []string
	if input.UserName != "" {
		fields = append(fields, "user_name")
	}
	if input.JobTitle != "" {
		fields = append(fields, "job_title")
	}
	if input.BusinessName != "" {
		fields = append(fields, "business_name")
	}
	if input.Industry != "" {
		fields = append(fields, "industry")
	}
	if input.BusinessSize != "" {
		fields = append(fields, "business_size")
	}
	if input.UserRole != "" {
		fields = append(fields, "user_role")
	}
	if len(input.KeyWorkflows) > 0 {
		fields = append(fields, "key_workflows")
	}
	if len(input.DailyActivities) > 0 {
		fields = append(fields, "daily_activities")
	}
	if len(input.PainPoints) > 0 {
		fields = append(fields, "pain_points")
	}
	if len(input.Bottlenecks) > 0 {
		fields = append(fields, "bottlenecks")
	}
	if len(input.ManualTasks) > 0 {
		fields = append(fields, "manual_tasks")
	}
	if len(input.AutomationGoals) > 0 {
		fields = append(fields, "automation_goals")
	}
	if len(input.CurrentSoftware) > 0 {
		fields = append(fields, "current_software")
	}
	if len(input.ExistingAutomation) > 0 {
		fields = append(fields, "existing_automation")
	}
	if input.AdditionalNotes != "" {
		fields = append(fields, "additional_notes")
	}
	return fields
}

func buildUnderstandingStatus(u database.BusinessUnderstanding) UnderstandingStatus {
	return UnderstandingStatus{
		UserName:             u.UserName != nil && *u.UserName != "",
		JobTitle:             u.JobTitle != nil && *u.JobTitle != "",
		BusinessName:         u.BusinessName != nil && *u.BusinessName != "",
		Industry:             u.Industry != nil && *u.Industry != "",
		BusinessSize:         u.BusinessSize != nil && *u.BusinessSize != "",
		UserRole:             u.UserRole != nil && *u.UserRole != "",
		KeyWorkflowsCount:    len(jsonToStringArray(u.KeyWorkflows)),
		DailyActivitiesCount: len(jsonToStringArray(u.DailyActivities)),
		PainPointsCount:      len(jsonToStringArray(u.PainPoints)),
		BottlenecksCount:     len(jsonToStringArray(u.Bottlenecks)),
		ManualTasksCount:     len(jsonToStringArray(u.ManualTasks)),
		AutomationGoalsCount: len(jsonToStringArray(u.AutomationGoals)),
		CurrentSoftwareCount: len(jsonToStringArray(u.CurrentSoftware)),
		ExistingAutomationCount: len(jsonToStringArray(u.ExistingAutomation)),
		AdditionalNotes:      u.AdditionalNotes != nil && *u.AdditionalNotes != "",
	}
}

func generateNextStepsMessage(status UnderstandingStatus) string {
	var missing []string

	// Priority 1: Basic identity
	if !status.UserName {
		missing = append(missing, "your name")
	}
	if !status.BusinessName {
		missing = append(missing, "your business name")
	}
	if !status.Industry {
		missing = append(missing, "your industry")
	}

	// Priority 2: Role and context
	if !status.JobTitle {
		missing = append(missing, "your job title")
	}
	if !status.UserRole {
		missing = append(missing, "your role in the organization (decision maker, implementer, end user)")
	}
	if !status.BusinessSize {
		missing = append(missing, "your company size")
	}

	// Priority 3: Workflows and operations
	if status.KeyWorkflowsCount == 0 {
		missing = append(missing, "your key business workflows")
	}
	if status.DailyActivitiesCount == 0 {
		missing = append(missing, "your daily activities")
	}

	// Priority 4: Pain points and goals
	if status.PainPointsCount == 0 {
		missing = append(missing, "your current pain points or challenges")
	}
	if status.AutomationGoalsCount == 0 {
		missing = append(missing, "your automation goals")
	}

	// Priority 5: Technical context
	if status.CurrentSoftwareCount == 0 {
		missing = append(missing, "the software tools you currently use")
	}

	if len(missing) == 0 {
		return "I have a comprehensive understanding of your business. We can proceed with generating personalized AI recommendations."
	}

	if len(missing) <= 2 {
		return fmt.Sprintf("To complete the picture, I'd love to learn about %s.", strings.Join(missing, " and "))
	}

	return fmt.Sprintf("To better understand your needs, could you tell me about %s, and %s?",
		strings.Join(missing[:len(missing)-1], ", "),
		missing[len(missing)-1])
}

func hasMinimumDataForReport(status UnderstandingStatus) bool {
	// Require at least: business name OR industry, AND some workflows or pain points
	hasBasicContext := status.BusinessName || status.Industry
	hasWorkContext := status.KeyWorkflowsCount > 0 || status.PainPointsCount > 0 || status.AutomationGoalsCount > 0
	return hasBasicContext && hasWorkContext
}

func generateDataGapsMessage(status UnderstandingStatus) string {
	var gaps []string

	if !status.BusinessName && !status.Industry {
		gaps = append(gaps, "your business or industry")
	}
	if status.KeyWorkflowsCount == 0 && status.PainPointsCount == 0 {
		gaps = append(gaps, "your workflows or challenges")
	}
	if status.AutomationGoalsCount == 0 {
		gaps = append(gaps, "your automation goals")
	}

	return fmt.Sprintf("I need more information to generate a meaningful report. Could you tell me about %s?",
		strings.Join(gaps, ", "))
}

func buildBusinessContextPrompt(ctx *BusinessContext) string {
	if ctx == nil {
		return ""
	}

	var parts []string

	// User and business identity
	if ctx.UserName != "" || ctx.BusinessName != "" || ctx.Industry != "" {
		var identity []string
		if ctx.UserName != "" {
			identity = append(identity, fmt.Sprintf("User: %s", ctx.UserName))
		}
		if ctx.JobTitle != "" {
			identity = append(identity, fmt.Sprintf("Role: %s", ctx.JobTitle))
		}
		if ctx.BusinessName != "" {
			identity = append(identity, fmt.Sprintf("Company: %s", ctx.BusinessName))
		}
		if ctx.Industry != "" {
			identity = append(identity, fmt.Sprintf("Industry: %s", ctx.Industry))
		}
		if ctx.BusinessSize != "" {
			identity = append(identity, fmt.Sprintf("Size: %s employees", ctx.BusinessSize))
		}
		parts = append(parts, strings.Join(identity, " | "))
	}

	// Key workflows
	if len(ctx.KeyWorkflows) > 0 {
		parts = append(parts, fmt.Sprintf("Key Workflows: %s", strings.Join(ctx.KeyWorkflows, ", ")))
	}

	// Pain points
	if len(ctx.PainPoints) > 0 {
		parts = append(parts, fmt.Sprintf("Pain Points: %s", strings.Join(ctx.PainPoints, ", ")))
	}

	// Automation goals
	if len(ctx.AutomationGoals) > 0 {
		parts = append(parts, fmt.Sprintf("Automation Goals: %s", strings.Join(ctx.AutomationGoals, ", ")))
	}

	// Current software
	if len(ctx.CurrentSoftware) > 0 {
		parts = append(parts, fmt.Sprintf("Current Tools: %s", strings.Join(ctx.CurrentSoftware, ", ")))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf(`
## User Business Context
%s

Use this context to provide personalized recommendations and tailor your responses to their specific business needs.
`, strings.Join(parts, "\n"))
}

// Utility functions for JSON array handling

func jsonToStringArray(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return []string{}
	}
	return arr
}

func stringArrayToJSON(arr []string) []byte {
	if arr == nil {
		arr = []string{}
	}
	data, _ := json.Marshal(arr)
	return data
}

func mergeStringArrays(existing, new []string) []string {
	if len(new) == 0 {
		return existing
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(existing)+len(new))

	for _, s := range existing {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	for _, s := range new {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
