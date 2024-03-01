package mcssh

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/apex/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/scp"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcssh/mc"
	"github.com/materials-commons/hydra/pkg/mcssh/mcscp"
	"github.com/materials-commons/hydra/pkg/mcssh/mcsftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Server struct {
	db             *gorm.DB
	server         *ssh.Server
	mcfsRoot       string
	userStore      stor.UserStor
	sshHost        string
	sshPort        string
	sshHostkeyPath string
}

func MustInitializeServer(db *gorm.DB) *Server {
	server := &Server{db: db}
	server.init()
	return server
}

func (s *Server) init() {
	incompleteConfiguration := false

	//dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
	//if dotenvFilePath == "" {
	//	log.Fatalf("MC_DOTENV_PATH not set or blank")
	//}
	//
	//if err := gotenv.Load(dotenvFilePath); err != nil {
	//	log.Fatalf("Failed loading configuration file %s: %s", dotenvFilePath, err)
	//}

	s.mcfsRoot = os.Getenv("MCFS_DIR")
	if s.mcfsRoot == "" {
		log.Errorf("MCFS_DIR is not set or blank")
		incompleteConfiguration = true
	}

	if s.sshPort = os.Getenv("MCSSHD_PORT"); s.sshPort == "" {
		log.Errorf("MCSSHD_PORT is not set or blank")
		incompleteConfiguration = true
	}

	if s.sshHost = os.Getenv("MCSSHD_HOST"); s.sshHost == "" {
		log.Errorf("MCSSHD_HOST is not set or blank")
		incompleteConfiguration = true
	}

	s.sshHostkeyPath = os.Getenv("MCSSHD_HOST_KEY_PATH")

	switch {
	case s.sshHostkeyPath == "":
		log.Errorf("MCSSHD_HOST_KEY_PATH is not set or blank")
		incompleteConfiguration = true
	default:
		if _, err := os.Stat(s.sshHostkeyPath); err != nil {
			log.Errorf("MCSSHD_HOST_KEY_PATH file (%s) does not exist: %s", s.sshHostkeyPath, err)
			incompleteConfiguration = true
		}
	}

	if incompleteConfiguration {
		log.Fatalf("One or more required variables not configured, exiting.")
	}
}

func (s *Server) Start() error {
	var err error
	stores := mc.NewGormStores(s.db, s.mcfsRoot)
	s.userStore = stor.NewGormUserStor(s.db)
	handler := mcscp.NewMCFSHandler(stores, s.mcfsRoot)
	s.server, err = wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", s.sshHost, s.sshPort)),
		wish.WithPasswordAuth(s.passwordHandler),
		wish.WithHostKeyPath(s.sshHostkeyPath),
		wish.WithMiddleware(scp.Middleware(handler, handler)),
	)

	if err != nil {
		return fmt.Errorf("failed created SSH Server: %s", err)
	}

	// SFTP is a subsystem, so rather than being handled as middleware we have to set
	// the subsystem handler.
	s.server.SubsystemHandlers = make(map[string]ssh.SubsystemHandler)
	s.server.SubsystemHandlers["sftp"] = func(session ssh.Session) {
		user := session.Context().Value("mcuser").(*mcmodel.User)
		h := mcsftp.NewMCFSHandler(user, stores, s.mcfsRoot)
		server := sftp.NewRequestServer(session, h)
		if err := server.Serve(); err == io.EOF {
			_ = server.Close()
		} else if err != nil {
			log.Errorf("sftp server completed with error: %s", err)
		}
	}

	return nil
}

func (s *Server) passwordHandler(context ssh.Context, password string) bool {
	userSlug := context.User()
	user, err := s.userStore.GetUserBySlug(userSlug)
	if err != nil {
		log.Errorf("Invalid user slug %q: %s", userSlug, err)
		return false
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return false
	}

	context.SetValue("mcuser", user)

	return true
}

func (s *Server) Stop(ctx context.Context) error {
	var err error
	if err = s.server.Shutdown(ctx); err != nil {
		log.Errorf("Error shutting down SSH Service: %s", err)
	}

	return err
}
