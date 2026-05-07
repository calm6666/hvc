package mysql

import "time"

// AdminUserRecord 表示管理员用户表映射。
type AdminUserRecord struct {
	AdminUserID  uint64    `gorm:"column:admin_user_id;primaryKey"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash"`
	PasswordSalt string    `gorm:"column:password_salt"`
	DisplayName  string    `gorm:"column:display_name"`
	Status       int       `gorm:"column:status"`
	LastLoginAt  time.Time `gorm:"column:last_login_at"`
	LastLoginIP  string    `gorm:"column:last_login_ip"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (AdminUserRecord) TableName() string { return "t_admin_user" }

// AdminSessionRecord 表示管理员会话表映射。
type AdminSessionRecord struct {
	SessionID     uint64    `gorm:"column:session_id;primaryKey"`
	AdminUserID   uint64    `gorm:"column:admin_user_id"`
	SessionToken  string    `gorm:"column:session_token"`
	SessionStatus int       `gorm:"column:session_status"`
	LoginIP       string    `gorm:"column:login_ip"`
	UserAgent     string    `gorm:"column:user_agent"`
	ExpireAt      time.Time `gorm:"column:expire_at"`
	LastSeenAt    time.Time `gorm:"column:last_seen_at"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (AdminSessionRecord) TableName() string { return "t_admin_session" }

// AdminAuditLogRecord 表示管理员审计表映射。
type AdminAuditLogRecord struct {
	AuditLogID       uint64    `gorm:"column:audit_log_id;primaryKey"`
	AdminUserID      uint64    `gorm:"column:admin_user_id"`
	Username         string    `gorm:"column:username"`
	ActionName       string    `gorm:"column:action_name"`
	TargetType       string    `gorm:"column:target_type"`
	TargetID         string    `gorm:"column:target_id"`
	RequestID        string    `gorm:"column:request_id"`
	RequestIP        string    `gorm:"column:request_ip"`
	RequestUserAgent string    `gorm:"column:request_user_agent"`
	ResultCode       int       `gorm:"column:result_code"`
	ResultMessage    string    `gorm:"column:result_message"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (AdminAuditLogRecord) TableName() string { return "t_admin_audit_log" }

// AdminRoleRecord 表示后台角色表映射。
type AdminRoleRecord struct {
	RoleID    uint64    `gorm:"column:role_id;primaryKey"`
	RoleKey   string    `gorm:"column:role_key"`
	RoleName  string    `gorm:"column:role_name"`
	RoleDesc  string    `gorm:"column:role_desc"`
	Status    int       `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (AdminRoleRecord) TableName() string { return "t_admin_role" }

// AdminPermissionRecord 表示权限点表映射。
type AdminPermissionRecord struct {
	PermID    uint64    `gorm:"column:perm_id;primaryKey"`
	PermKey   string    `gorm:"column:perm_key"`
	PermName  string    `gorm:"column:perm_name"`
	PermDesc  string    `gorm:"column:perm_desc"`
	Module    string    `gorm:"column:module"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AdminPermissionRecord) TableName() string { return "t_admin_permission" }

// AdminUserRoleRecord 表示管理员用户与角色关联表映射。
type AdminUserRoleRecord struct {
	ID        uint64    `gorm:"column:id;primaryKey"`
	UserID    uint64    `gorm:"column:user_id"`
	RoleID    uint64    `gorm:"column:role_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AdminUserRoleRecord) TableName() string { return "t_admin_user_role" }

// AdminRolePermissionRecord 表示角色与权限点关联表映射。
type AdminRolePermissionRecord struct {
	ID        uint64    `gorm:"column:id;primaryKey"`
	RoleID    uint64    `gorm:"column:role_id"`
	PermID    uint64    `gorm:"column:perm_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AdminRolePermissionRecord) TableName() string { return "t_admin_role_permission" }
