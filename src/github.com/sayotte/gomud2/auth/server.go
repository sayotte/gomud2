package auth

import (
	"errors"
	"fmt"
	uuid2 "github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/uuid"
	"golang.org/x/crypto/bcrypt"
	"regexp"
)

const (
	targetCost = 11
)

var (
	BadUserOrPassphraseError = errors.New("Bad username or passphrase")
	validUsernameRE          = regexp.MustCompile("^[a-zA-Z]+$")
)

type Server struct {
	AccountDatabaseFile string
	database            *authDatabase
}

func (s *Server) Start() error {
	s.database = &authDatabase{
		accountDBFile: s.AccountDatabaseFile,
	}
	return s.database.load()
}

func (s *Server) CreateAccount(user, pass string) error {
	if !validUsernameRE.MatchString(user) {
		return errors.New("Invalid username, must be only alphabet characters.")
	}
	_, _, err := s.database.getEntryByUsername(user)
	if err == nil {
		return errors.New("Username is already taken, choose another.")
	}

	ent := dbEntry{
		AuthZDescriptor: AuthZDescriptor{
			CreateCharacter: false,
			MayLogin:        true,
		},
		Username: user,
	}
	newID := uuid.NewId()

	return s.storeAccountWithPassphrase(newID, pass, ent)
}

func (s *Server) DoLogin(user, pass string) (uuid2.UUID, *AuthZDescriptor, error) {
	id, dbEntry, err := s.database.getEntryByUsername(user)
	if err != nil {
		return uuid2.Nil, nil, BadUserOrPassphraseError
	}

	hashErr := bcrypt.CompareHashAndPassword([]byte(dbEntry.PasswordHash), []byte(pass))
	if hashErr != nil {
		return uuid2.Nil, nil, BadUserOrPassphraseError
	}

	// while we're here, if the cost factor for the stored password isn't up to
	// snuff let's re-hash the supplied cleartext pass to a more powerful cost
	// factor
	cost, err := bcrypt.Cost([]byte(dbEntry.PasswordHash))
	if err != nil {
		return uuid2.Nil, nil, fmt.Errorf("bcrypt.Cost(): %s", err)
	}
	if cost < targetCost {
		err = s.storeAccountWithPassphrase(id, pass, dbEntry)
		if err != nil {
			return uuid2.Nil, nil, err
		}
	}

	return id, &dbEntry.AuthZDescriptor, nil
}

func (s *Server) GetActorIDsForAccountID(acctID uuid2.UUID) []uuid2.UUID {
	ent, err := s.database.getEntryByID(acctID)
	if err != nil {
		return nil
	}
	return ent.Actors
}

func (s *Server) AddActorIDToAccountID(acctID, actorID uuid2.UUID) error {
	ent, err := s.database.getEntryByID(acctID)
	if err != nil {
		return err
	}
	ent.Actors = append(ent.Actors, actorID)
	return s.database.putEntry(acctID, ent)
}

func (s *Server) storeAccountWithPassphrase(id uuid2.UUID, pass string, ent dbEntry) error {
	newHash, err := bcrypt.GenerateFromPassword([]byte(pass), targetCost)
	if err != nil {
		return fmt.Errorf("bcrypt.GenerateFromPassword(): %s", err)
	}
	ent.PasswordHash = string(newHash)
	return s.database.putEntry(id, ent)
}
