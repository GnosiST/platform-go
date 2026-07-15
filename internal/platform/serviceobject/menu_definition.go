package serviceobject

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

const MaximumMenuParameters = 32

var forbiddenMenuParameterWordPattern = regexp.MustCompile(`\b(?:select|insert|update|delete|drop|alter|create|truncate|merge|exec|execute|union|datasource|shard|database|schema|sql|script|expression)\b`)

type MenuNodeType string

const (
	MenuNodeTypeDirectory MenuNodeType = "directory"
	MenuNodeTypePage      MenuNodeType = "page"
)

type MenuParameterType string

const (
	MenuParameterTypeString  MenuParameterType = "string"
	MenuParameterTypeNumber  MenuParameterType = "number"
	MenuParameterTypeBoolean MenuParameterType = "boolean"
)

type MenuOpenMode string

const (
	MenuOpenModeSameTab MenuOpenMode = "same-tab"
	MenuOpenModeNewTab  MenuOpenMode = "new-tab"
)

type MenuParameter struct {
	Key   string            `json:"key"`
	Type  MenuParameterType `json:"type"`
	Value any               `json:"value"`
}

type MenuNode struct {
	Code              string          `json:"code"`
	ParentCode        string          `json:"parentCode"`
	NodeType          MenuNodeType    `json:"nodeType"`
	TitleZH           string          `json:"titleZh"`
	TitleEN           string          `json:"titleEn"`
	DescriptionZH     string          `json:"descriptionZh"`
	DescriptionEN     string          `json:"descriptionEn"`
	Status            string          `json:"status"`
	Icon              string          `json:"icon"`
	SortOrder         int             `json:"sortOrder"`
	Route             string          `json:"route"`
	ComponentKey      string          `json:"componentKey"`
	ResourceCode      string          `json:"resourceCode"`
	External          bool            `json:"external"`
	ExternalURL       string          `json:"externalUrl"`
	OpenMode          MenuOpenMode    `json:"openMode"`
	Parameters        []MenuParameter `json:"parameters"`
	CacheEnabled      bool            `json:"cacheEnabled"`
	Hidden            bool            `json:"hidden"`
	ActiveMenuCode    string          `json:"activeMenuCode"`
	BreadcrumbVisible bool            `json:"breadcrumbVisible"`
}

type PageButton struct {
	MenuCode       string `json:"menuCode"`
	ButtonKey      string `json:"buttonKey"`
	LabelZH        string `json:"labelZh"`
	LabelEN        string `json:"labelEn"`
	Action         string `json:"action"`
	SortOrder      int    `json:"sortOrder"`
	Status         string `json:"status"`
	PermissionCode string `json:"permissionCode"`
}

type MenuDefinition struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	UpdatedAt   string       `json:"updatedAt"`
	Node        MenuNode     `json:"node"`
	Buttons     []PageButton `json:"buttons"`
}

func normalizeMenuDefinition(value any) (MenuDefinition, error) {
	switch typed := value.(type) {
	case MenuDefinition:
		if validateMenuDefinition(typed) != nil {
			return MenuDefinition{}, ErrRequestInvalid
		}
		return cloneMenuDefinition(typed), nil
	case map[string]any:
		payload, err := json.Marshal(typed)
		if err != nil {
			return MenuDefinition{}, ErrRequestInvalid
		}
		var definition MenuDefinition
		if err := decodeStrictJSON(bytes.NewReader(payload), &definition); err != nil || validateMenuDefinition(definition) != nil {
			return MenuDefinition{}, ErrRequestInvalid
		}
		return cloneMenuDefinition(definition), nil
	default:
		return MenuDefinition{}, ErrRequestInvalid
	}
}

func validateMenuDefinition(definition MenuDefinition) error {
	if !validMenuValue(definition.ID) || strings.TrimSpace(definition.Name) == "" {
		return ErrRequestInvalid
	}
	node := definition.Node
	if !validMenuValue(node.Code) || node.ParentCode != "" && !validMenuValue(node.ParentCode) ||
		strings.TrimSpace(node.TitleZH) == "" || strings.TrimSpace(node.TitleEN) == "" ||
		node.Status != "enabled" && node.Status != "disabled" {
		return ErrRequestInvalid
	}
	if node.ActiveMenuCode != "" && (!validMenuValue(node.ActiveMenuCode) || node.ActiveMenuCode == node.Code) {
		return ErrRequestInvalid
	}
	switch node.NodeType {
	case MenuNodeTypeDirectory:
		if node.Route != "" || node.ComponentKey != "" || node.ResourceCode != "" || node.External || node.ExternalURL != "" || node.OpenMode != "" || len(node.Parameters) != 0 || len(definition.Buttons) != 0 {
			return ErrRequestInvalid
		}
	case MenuNodeTypePage:
		if err := validateMenuPageNode(node); err != nil {
			return err
		}
	default:
		return ErrRequestInvalid
	}
	buttonKeys := make(map[string]struct{}, len(definition.Buttons))
	permissionCodes := make(map[string]struct{}, len(definition.Buttons))
	for _, button := range definition.Buttons {
		if button.MenuCode != node.Code || !validMenuValue(button.ButtonKey) || strings.TrimSpace(button.LabelZH) == "" || strings.TrimSpace(button.LabelEN) == "" ||
			!validMenuValue(button.Action) || !validMenuValue(button.PermissionCode) || button.Status != "enabled" && button.Status != "disabled" {
			return ErrRequestInvalid
		}
		if _, duplicate := buttonKeys[button.ButtonKey]; duplicate {
			return ErrRequestInvalid
		}
		if _, duplicate := permissionCodes[button.PermissionCode]; duplicate {
			return ErrRequestInvalid
		}
		buttonKeys[button.ButtonKey] = struct{}{}
		permissionCodes[button.PermissionCode] = struct{}{}
	}
	return nil
}

func validateMenuPageNode(node MenuNode) error {
	if len(node.Parameters) > MaximumMenuParameters {
		return ErrRequestInvalid
	}
	parameterKeys := make(map[string]struct{}, len(node.Parameters))
	for _, parameter := range node.Parameters {
		if !validMenuValue(parameter.Key) || forbiddenMenuParameterKey(parameter.Key) {
			return ErrRequestInvalid
		}
		if _, duplicate := parameterKeys[parameter.Key]; duplicate {
			return ErrRequestInvalid
		}
		parameterKeys[parameter.Key] = struct{}{}
		switch parameter.Type {
		case MenuParameterTypeString:
			value, ok := parameter.Value.(string)
			if !ok || IsForbiddenMenuParameterStringValue(value) {
				return ErrRequestInvalid
			}
		case MenuParameterTypeNumber:
			switch parameter.Value.(type) {
			case json.Number, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			default:
				return ErrRequestInvalid
			}
		case MenuParameterTypeBoolean:
			if _, ok := parameter.Value.(bool); !ok {
				return ErrRequestInvalid
			}
		default:
			return ErrRequestInvalid
		}
	}
	if node.External {
		parsed, err := url.Parse(strings.TrimSpace(node.ExternalURL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || node.Route != "" || node.ComponentKey != "" || node.ResourceCode != "" ||
			node.OpenMode != MenuOpenModeSameTab && node.OpenMode != MenuOpenModeNewTab {
			return ErrRequestInvalid
		}
		return nil
	}
	if !strings.HasPrefix(node.Route, "/") || strings.HasPrefix(node.Route, "//") || strings.ContainsAny(node.Route, "{}*") || strings.Contains(node.Route, ":") ||
		strings.TrimSpace(node.ComponentKey) == "" || node.ExternalURL != "" || node.OpenMode != "" {
		return ErrRequestInvalid
	}
	return nil
}

func validMenuValue(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 191
}

func forbiddenMenuParameterKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "datasource", "shard", "database", "schema", "sql", "script", "expression", "route-template", "physical-database", "physical-schema", "physical-routing":
		return true
	default:
		return false
	}
}

// IsForbiddenMenuParameterStringValue reports whether a static menu parameter carries executable or physical-routing input.
func IsForbiddenMenuParameterStringValue(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if forbiddenMenuParameterWordPattern.MatchString(normalized) {
		return true
	}
	for _, marker := range []string{
		"<script", "</script", "javascript:", "vbscript:", "data:text/html", "eval(", "function(",
		"${", "#{", "@{", "{{", "}}", "/:", "/{", "/*",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
}

func cloneMenuDefinition(definition MenuDefinition) MenuDefinition {
	definition.Node.Parameters = append([]MenuParameter(nil), definition.Node.Parameters...)
	definition.Buttons = append([]PageButton(nil), definition.Buttons...)
	return definition
}
