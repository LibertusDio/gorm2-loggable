package loggable

import (
	"encoding/json"
	"reflect"

	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

var im = newIdentityManager()

type UpdateDiff map[string]interface{}
type DiffObject struct {
	Old interface{} `json:"old"`
	New interface{} `json:"new"`
}

// Hook for after_query.
func (p *Plugin) trackEntity(scope *gorm.Scope) {
	if !isLoggable(scope.Value) || !isEnabled(scope.Value) {
		return
	}

	v := reflect.Indirect(reflect.ValueOf(scope.Value))

	pkName := scope.PrimaryField().Name
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			sv := reflect.Indirect(v.Index(i))
			el := sv.Interface()
			if !isLoggable(el) {
				continue
			}

			im.save(el, sv.FieldByName(pkName))
		}
		return
	}

	m := v.Interface()
	if !isLoggable(m) {
		return
	}

	im.save(scope.Value, scope.PrimaryKeyValue())
}

// Hook for after_create.
func (p *Plugin) addCreated(scope *gorm.Scope) {
	if isLoggable(scope.Value) && isEnabled(scope.Value) {
		_ = p.addRecord(scope, actionCreate)
	}
}

// Hook for after_update.
func (p *Plugin) addUpdated(scope *gorm.Scope) {
	if !isLoggable(scope.Value) || !isEnabled(scope.Value) {
		return
	}

	if p.opts.lazyUpdate {
		record, err := p.GetLastRecord(interfaceToString(scope.PrimaryKeyValue()), false)
		if err == nil {
			if isEqual(record.RawObject, scope.Value, p.opts.lazyUpdateFields...) {
				return
			}
		}
	}

	_ = p.addUpdateRecord(scope, p.opts)
}

// Hook for after_delete.
func (p *Plugin) addDeleted(scope *gorm.Scope) {
	if isLoggable(scope.Value) && isEnabled(scope.Value) {
		_ = p.addRecord(scope, actionDelete)
	}
}

func (p *Plugin) addUpdateRecord(scope *gorm.Scope, opts options) error {
	cl, err := newChangeLog(scope, actionUpdate)
	if err != nil {
		return err
	}

	if opts.computeDiff {
		diff := computeUpdateDiff(scope)

		if diff != nil {
			jd, err := json.Marshal(diff)
			if err != nil {
				return err
			}

			cl.RawDiff = string(jd)
		}
	}

	return scope.DB().Table(p.tablename).Create(cl).Error
}

func newChangeLog(scope *gorm.Scope, action string) (*ChangeLog, error) {
	rawObject, err := json.Marshal(scope.Value)
	if err != nil {
		return nil, err
	}
	id := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	ui, ok := scope.Get(LoggableUserTag)
	var u *User
	if !ok {
		u = &User{"unknown", "system", "unknown"}
	} else {
		u, ok = ui.(*User)
		if !ok {
			u = &User{"unknown", "system", "unknown"}
		}
	}
	us := `{"name":"unknown","id":"system","class":"unknown"}`
	ub, err := json.Marshal(u)
	if err == nil {
		us = string(ub)
	}

	return &ChangeLog{
		ID:         id.String(),
		Action:     action,
		ObjectID:   interfaceToString(scope.PrimaryKeyValue()),
		ObjectType: scope.TableName(),
		RawObject:  string(rawObject),
		RawMeta:    string(fetchChangeLogMeta(scope)),
		RawDiff:    "null",
		CreatedBy:  us,
	}, nil
}

// Writes new change log row to db.
func (p *Plugin) addRecord(scope *gorm.Scope, action string) error {
	cl, err := newChangeLog(scope, action)
	if err != nil {
		return nil
	}

	return scope.DB().Table(p.tablename).Create(cl).Error
}

func computeUpdateDiff(scope *gorm.Scope) UpdateDiff {
	old, ok := scope.Get(LoggablePrevVersion)
	if !ok {
		return nil
	}

	ov := reflect.Indirect(reflect.ValueOf(old))
	nv := reflect.Indirect(reflect.ValueOf(scope.Value))
	names := getLoggableFieldNames(old)

	diff := make(UpdateDiff)

	for _, name := range names {
		ofv := ov.FieldByName(name).Interface()
		nfv := nv.FieldByName(name).Interface()
		if !reflect.DeepEqual(ofv, nfv) {
			diff[ToSnakeCaseRegEx(name)] = DiffObject{
				Old: ofv,
				New: nfv,
			}
		}
	}

	return diff
}
