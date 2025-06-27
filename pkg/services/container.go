package services

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/log"
	_ "github.com/mattn/go-sqlite3"

	// Required by ent.
	_ "github.com/liukeshao/echo-template/ent/runtime"
)

// Container contains all services used by the application and provides an easy way to handle dependency
// injection including within tests.
type Container struct {
	// Web stores the web framework.
	Web *echo.Echo

	// Config stores the application configuration.
	Config *config.Config

	// Database stores the connection to the database.
	Database *sql.DB

	// ORM stores a client to the ORM.
	ORM *ent.Client

	Auth *AuthService
	User *UserService
}

// NewContainer creates and initializes a new Container.
func NewContainer() *Container {
	c := new(Container)
	c.initConfig()
	c.initWeb()
	c.initDatabase()
	c.initORM()
	c.initAuth()
	return c
}

// Shutdown gracefully shuts the Container down and disconnects all connections.
func (c *Container) Shutdown() error {
	// Shutdown the web server.
	webCtx, webCancel := context.WithTimeout(context.Background(), c.Config.HTTP.ShutdownTimeout)
	defer webCancel()
	if err := c.Web.Shutdown(webCtx); err != nil {
		return err
	}

	// Shutdown the ORM.
	if err := c.ORM.Close(); err != nil {
		return err
	}

	// Shutdown the database.
	if err := c.Database.Close(); err != nil {
		return err
	}

	return nil
}

// initConfig initializes configuration.
func (c *Container) initConfig() {
	cfg, err := config.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}
	c.Config = &cfg

	// Configure logging.
	log.Setup(c.Config)
}

// initWeb initializes the web framework.
func (c *Container) initWeb() {
	c.Web = echo.New()
	c.Web.HideBanner = true
}

// initDatabase initializes the database.
func (c *Container) initDatabase() {
	var err error
	c.Database, err = openDB(c.Config.Database.Driver, c.Config.Database.Connection)
	if err != nil {
		panic(err)
	}
}

// initORM initializes the ORM.
func (c *Container) initORM() {
	drv := entsql.OpenDB(c.Config.Database.Driver, c.Database)
	c.ORM = ent.NewClient(ent.Driver(drv))

	// Run the auto migration tool.
	if err := c.ORM.Schema.Create(context.Background()); err != nil {
		panic(err)
	}
}

func (c *Container) initAuth() {
	jwtConfig := NewJWTConfigFromConfig(c.Config.JWT)
	c.Auth = NewAuthService(c.ORM, jwtConfig)
}

// openDB opens a database connection.
func openDB(driver, connection string) (*sql.DB, error) {
	if driver == "sqlite3" {
		// Helper to automatically create the directories that the specified sqlite file
		// should reside in, if one.
		d := strings.Split(connection, "/")
		if len(d) > 1 {
			dirpath := strings.Join(d[:len(d)-1], "/")

			if err := os.MkdirAll(dirpath, 0755); err != nil {
				return nil, err
			}
		}
	}

	return sql.Open(driver, connection)
}
