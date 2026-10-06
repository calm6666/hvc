package admin

import (
	"sort"
	"strings"
	"time"

	"hvc/internal/audit"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/opslog"
)

type adminUserView struct {
	AdminUserID uint64     `json:"admin_user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Status      int        `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP string     `json:"last_login_ip,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type adminRoleView struct {
	RoleID    uint64    `json:"role_id"`
	RoleKey   string    `json:"role_key"`
	RoleName  string    `json:"role_name"`
	RoleDesc  string    `json:"role_desc"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type adminPermissionView struct {
	PermID    uint64    `json:"perm_id"`
	PermKey   string    `json:"perm_key"`
	PermName  string    `json:"perm_name"`
	PermDesc  string    `json:"perm_desc"`
	Module    string    `json:"module"`
	CreatedAt time.Time `json:"created_at"`
}

type adminMenuNode struct {
	ID            uint64          `json:"id"`
	MenuID        uint64          `json:"menu_id"`
	ParentID      uint64          `json:"parent_id"`
	MenuKey       string          `json:"menu_key"`
	MenuName      string          `json:"menu_name"`
	RoutePath     string          `json:"route_path,omitempty"`
	Component     string          `json:"component,omitempty"`
	IconName      string          `json:"icon_name,omitempty"`
	MenuType      string          `json:"menu_type"`
	PermissionKey string          `json:"permission_key,omitempty"`
	SortNo        int             `json:"sort_no"`
	Hidden        bool            `json:"hidden"`
	Status        int             `json:"status"`
	Children      []adminMenuNode `json:"children,omitempty"`
}

type auditLogView struct {
	AuditLogID       uint64    `json:"audit_log_id"`
	AdminUserID      uint64    `json:"admin_user_id"`
	Username         string    `json:"username"`
	ActionName       string    `json:"action_name"`
	TargetType       string    `json:"target_type"`
	TargetID         string    `json:"target_id"`
	RequestID        string    `json:"request_id"`
	RequestIP        string    `json:"request_ip"`
	RequestUserAgent string    `json:"request_user_agent"`
	ResultCode       int       `json:"result_code"`
	ResultMessage    string    `json:"result_message"`
	CreatedAt        time.Time `json:"created_at"`
}

type runtimeLogView struct {
	LogID      uint64         `json:"log_id"`
	Service    string         `json:"service_name"`
	Level      string         `json:"log_level"`
	ActionName string         `json:"action_name"`
	Fields     map[string]any `json:"fields"`
	LoggedAt   time.Time      `json:"logged_at"`
}

type permissionTreeNode struct {
	ID         string               `json:"id"`
	ParentID   string               `json:"parent_id"`
	PermID     uint64               `json:"perm_id,omitempty"`
	Key        string               `json:"key"`
	Label      string               `json:"label"`
	NodeType   string               `json:"node_type"`
	Module     string               `json:"module,omitempty"`
	Permission *adminPermissionView `json:"permission,omitempty"`
	Children   []permissionTreeNode `json:"children,omitempty"`
}

func toAdminUserView(record mysql.AdminUserRecord) adminUserView {
	view := adminUserView{
		AdminUserID: record.AdminUserID,
		Username:    record.Username,
		DisplayName: record.DisplayName,
		Status:      record.Status,
		LastLoginIP: strings.TrimSpace(record.LastLoginIP),
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	if !record.LastLoginAt.IsZero() {
		lastLoginAt := record.LastLoginAt
		view.LastLoginAt = &lastLoginAt
	}
	return view
}

func toAdminRoleView(record mysql.AdminRoleRecord) adminRoleView {
	return adminRoleView{
		RoleID:    record.RoleID,
		RoleKey:   record.RoleKey,
		RoleName:  record.RoleName,
		RoleDesc:  record.RoleDesc,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func toAdminMenuNode(record mysql.AdminMenuRecord) adminMenuNode {
	return adminMenuNode{
		ID:            record.MenuID,
		MenuID:        record.MenuID,
		ParentID:      record.ParentID,
		MenuKey:       record.MenuKey,
		MenuName:      record.MenuName,
		RoutePath:     record.RoutePath,
		Component:     record.Component,
		IconName:      record.IconName,
		MenuType:      record.MenuType,
		PermissionKey: record.PermissionKey,
		SortNo:        record.SortNo,
		Hidden:        record.Hidden,
		Status:        record.Status,
	}
}

func toAdminPermissionView(record mysql.AdminPermissionRecord) adminPermissionView {
	return adminPermissionView{
		PermID:    record.PermID,
		PermKey:   record.PermKey,
		PermName:  record.PermName,
		PermDesc:  record.PermDesc,
		Module:    record.Module,
		CreatedAt: record.CreatedAt,
	}
}

func toAuditLogView(item audit.Item) auditLogView {
	return auditLogView{
		AuditLogID:       item.AuditLogID,
		AdminUserID:      item.AdminUserID,
		Username:         item.Username,
		ActionName:       item.ActionName,
		TargetType:       item.TargetType,
		TargetID:         item.TargetID,
		RequestID:        item.RequestID,
		RequestIP:        item.RequestIP,
		RequestUserAgent: item.RequestUserAgent,
		ResultCode:       item.ResultCode,
		ResultMessage:    item.ResultMessage,
		CreatedAt:        item.CreatedAt,
	}
}

func toRuntimeLogView(item opslog.Item) runtimeLogView {
	return runtimeLogView{
		LogID:      item.LogID,
		Service:    item.Service,
		Level:      item.Level,
		ActionName: item.ActionName,
		Fields:     item.Fields,
		LoggedAt:   item.LoggedAt,
	}
}

func buildMenuTree(records []mysql.AdminMenuRecord) []adminMenuNode {
	if len(records) == 0 {
		return nil
	}
	nodes := make(map[uint64]*adminMenuNode, len(records))
	for _, record := range records {
		record := record
		nodes[record.MenuID] = &adminMenuNode{
			MenuID:        record.MenuID,
			ID:            record.MenuID,
			ParentID:      record.ParentID,
			MenuKey:       record.MenuKey,
			MenuName:      record.MenuName,
			RoutePath:     record.RoutePath,
			Component:     record.Component,
			IconName:      record.IconName,
			MenuType:      record.MenuType,
			PermissionKey: record.PermissionKey,
			SortNo:        record.SortNo,
			Hidden:        record.Hidden,
			Status:        record.Status,
		}
	}
	roots := make([]adminMenuNode, 0, len(records))
	for _, record := range records {
		node := nodes[record.MenuID]
		if node == nil {
			continue
		}
		if record.ParentID == 0 || nodes[record.ParentID] == nil {
			roots = append(roots, *node)
			continue
		}
		parent := nodes[record.ParentID]
		parent.Children = append(parent.Children, *node)
	}
	for _, root := range roots {
		sortMenuTreeChildren(nodes[root.MenuID])
	}
	sort.SliceStable(roots, func(i, j int) bool {
		if roots[i].SortNo == roots[j].SortNo {
			return roots[i].MenuID < roots[j].MenuID
		}
		return roots[i].SortNo < roots[j].SortNo
	})
	return rebuildTree(roots, nodes)
}

func sortMenuTreeChildren(node *adminMenuNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}
	sort.SliceStable(node.Children, func(i, j int) bool {
		if node.Children[i].SortNo == node.Children[j].SortNo {
			return node.Children[i].MenuID < node.Children[j].MenuID
		}
		return node.Children[i].SortNo < node.Children[j].SortNo
	})
	for i := range node.Children {
		child := node.Children[i]
		sortMenuTreeChildren(&child)
		node.Children[i] = child
	}
}

func rebuildTree(roots []adminMenuNode, index map[uint64]*adminMenuNode) []adminMenuNode {
	result := make([]adminMenuNode, 0, len(roots))
	for _, root := range roots {
		result = append(result, rebuildNode(root, index))
	}
	return result
}

func rebuildNode(node adminMenuNode, index map[uint64]*adminMenuNode) adminMenuNode {
	current := node
	if original := index[node.MenuID]; original != nil {
		current.Children = make([]adminMenuNode, 0, len(original.Children))
		for _, child := range original.Children {
			current.Children = append(current.Children, rebuildNode(child, index))
		}
	}
	return current
}

func buildPermissionTree(records []mysql.AdminPermissionRecord) []permissionTreeNode {
	type mutablePermissionNode struct {
		key        string
		label      string
		nodeType   string
		module     string
		permission *adminPermissionView
		children   map[string]*mutablePermissionNode
	}

	var freeze func(node *mutablePermissionNode, parentID string) permissionTreeNode
	freeze = func(node *mutablePermissionNode, parentID string) permissionTreeNode {
		result := permissionTreeNode{
			ID:       node.key,
			ParentID: parentID,
			Key:      node.key,
			Label:    node.label,
			NodeType: node.nodeType,
			Module:   node.module,
		}
		if node.permission != nil {
			perm := *node.permission
			result.PermID = perm.PermID
			result.Permission = &perm
		}
		if len(node.children) == 0 {
			return result
		}
		childKeys := make([]string, 0, len(node.children))
		for key := range node.children {
			childKeys = append(childKeys, key)
		}
		sort.Strings(childKeys)
		result.Children = make([]permissionTreeNode, 0, len(childKeys))
		for _, key := range childKeys {
			result.Children = append(result.Children, freeze(node.children[key], node.key))
		}
		return result
	}

	root := &mutablePermissionNode{children: make(map[string]*mutablePermissionNode)}
	for _, record := range records {
		module := strings.TrimSpace(record.Module)
		if module == "" {
			module = "default"
		}
		moduleNode := root.children[module]
		if moduleNode == nil {
			moduleNode = &mutablePermissionNode{
				key:      module,
				label:    module,
				nodeType: "module",
				module:   module,
				children: make(map[string]*mutablePermissionNode),
			}
			root.children[module] = moduleNode
		}

		current := moduleNode
		segments := strings.Split(strings.TrimSpace(record.PermKey), ".")
		start := 0
		if len(segments) > 0 && segments[0] == module {
			start = 1
		}
		for i := start; i < len(segments); i++ {
			segment := strings.TrimSpace(segments[i])
			if segment == "" {
				continue
			}
			nodeType := "group"
			if i == len(segments)-1 {
				nodeType = "permission"
			}
			child := current.children[segment]
			if child == nil {
				child = &mutablePermissionNode{
					key:      current.key + "." + segment,
					label:    segment,
					nodeType: nodeType,
					module:   module,
					children: make(map[string]*mutablePermissionNode),
				}
				current.children[segment] = child
			}
			if nodeType == "permission" {
				view := toAdminPermissionView(record)
				child.label = firstNonEmpty(record.PermName, segment)
				child.permission = &view
			}
			current = child
		}
	}

	moduleKeys := make([]string, 0, len(root.children))
	for key := range root.children {
		moduleKeys = append(moduleKeys, key)
	}
	sort.Strings(moduleKeys)
	result := make([]permissionTreeNode, 0, len(moduleKeys))
	for _, key := range moduleKeys {
		result = append(result, freeze(root.children[key], ""))
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
