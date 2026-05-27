package keenbase

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type SimpleBase struct {
	config      Config
	db          *sql.DB
	collections *CollectionStore
	records     *RecordStore
	auth        *AuthStore
	files       *FileStore
	Cron        *Cron
}

type Config struct {
	Port int

	DataDir string
}

func New() *SimpleBase {
	return WithConfig(Config{})
}

func WithConfig(config Config) *SimpleBase {
	if config.Port == 0 {
		config.Port = 8090
	}
	if config.DataDir == "" {
		config.DataDir = "./pb_data"
	}
	return &SimpleBase{config: config}
}

func (sb *SimpleBase) Start() error {
	if err := sb.init(); err != nil {
		return err
	}
	sb.Cron.Add("example-job", "* * * * *", func() {
		log.Println("Job-1 logs coming here")
	})
	sb.Cron.Add("example-job-2", "* * * * *", func() {
		log.Println("Job-2 logs coming here")
	})
	sb.Cron.SetInterval(2 * time.Second)
	log.Printf("keenbase started — data dir: %s, port: %d", sb.config.DataDir, sb.config.Port)
	return sb.serve()
}

func (sb *SimpleBase) init() error {
	if sb.db != nil {
		return nil
	}

	db, err := openDB(sb.config.DataDir)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	sb.db = db

	if err := bootstrap(db); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	sb.collections = newCollectionStore(db)
	sb.records = newRecordStore(db)
	sb.auth = newAuthStore(db, sb.records)
	sb.files = newFileStore(sb.config.DataDir)
	sb.Cron = NewCron()
	return nil
}

func (sb *SimpleBase) CreateSuperuser(email, password string) error {
	if err := sb.init(); err != nil {
		return err
	}

	col := &Collection{
		ID:   SuperusersCollectionID,
		Name: "_superusers",
		Type: CollectionTypeAuth,
		AuthOptions: &AuthOptions{
			PasswordAuth: PasswordAuthOptions{
				Enabled:        true,
				IdentityFields: []string{"email"},
			},
		},
	}

	_, err := sb.auth.CreateAuthRecord(col, email, password, nil)
	if err != nil {
		return fmt.Errorf("create superuser: %w", err)
	}

	log.Printf("superuser created: %s", email)
	return nil
}

func (sb *SimpleBase) DB() *sql.DB {
	return sb.db
}
