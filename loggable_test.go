package loggable

import (
	"fmt"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

type SomeType struct {
	gorm.Model
	Source string
	MetaModel
}

type MetaModel struct {
	createdBy string
	LoggableModel
}

func (m MetaModel) Meta() interface{} {
	return struct {
		CreatedBy string
	}{CreatedBy: m.createdBy}
}

func TestMain(m *testing.M) {
	database, err := gorm.Open(postgres.New(postgres.Config{
		DSN: fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=disable",
			"root",
			"keepitsimple",
			"localhost",
			5432,
			"loggable",
		),
		PreferSimpleProtocol: true,
	}))
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	database.Logger = logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)
	_, err = Register(database, "change_logs")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	err = database.AutoMigrate(SomeType{})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	db = database
	os.Exit(m.Run())
}

func TestTryModel(t *testing.T) {
	newmodel := SomeType{Source: time.Now().Format(time.Stamp)}
	newmodel.createdBy = "some user"
	err := db.Create(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(newmodel.ID)
	newmodel.Source = "updated field"
	err = db.Model(SomeType{}).Save(&newmodel).Error
	if err != nil {
		t.Fatal(err)
	}
}
