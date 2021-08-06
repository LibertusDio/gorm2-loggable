package loggable

const loggableTag = "gorm-loggable"
const LoggableUserTag = "gorm-loggable:user"

const (
	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

const DefaultTableName = "change_logs"
