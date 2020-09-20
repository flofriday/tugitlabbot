package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

const dbFile string = "data.db"

// NOTE: Dear developer only add new fields AT THE EMD to avoid breaking the
// code.
const (
	UserSetup int = iota
	UserNormal
)

// The Usertype specifies what a user of this bot is
type User struct {
	TelegramID  int64
	GitLabToken string
	LastChecked time.Time
	HasError    bool
	State       int
}

// Setup the database on the disk.
// This function must be called before using any other function from this file!
func InitUsers() error {
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Users"))
		return err
	})
	return err
}

// Create a new user and save it to Disk
func NewUser(telegramID int64) (User, error) {
	u := User{
		TelegramID:  telegramID,
		GitLabToken: "",
		State:       UserSetup,
		HasError:    false,
	}
	err := u.Save()
	return u, err
}

// Load a user from disk with a specified id
func LoadUser(telegramID int64) (User, error) {
	// Open the database
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return User{}, err
	}

	// Start a read transaction and close the database
	var rawJSON []byte
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Users"))
		v := b.Get([]byte(strconv.FormatInt(telegramID, 10)))
		rawJSON = make([]byte, len(v))
		copy(rawJSON, v)
		return nil
	})
	db.Close()
	if err != nil {
		return User{}, err
	}

	// Decode the json
	user := User{}
	err = json.Unmarshal(rawJSON, &user)
	if err != nil {

		// Since we can't parse the json, we asume that the user doeesn't
		// exist so we try to create it
		user, err := NewUser(telegramID)
		if err != nil {
			log.Print("[Error] Unable to create new user")
			return User{}, err
		}

		log.Printf("[Info] Created new user %v", telegramID)
		return user, nil
	}

	return user, nil
}

// Load all user
func LoadAllUsers() ([]User, error) {
	// Open the database
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Start the transaction
	cache := make(map[string][]byte)
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Users"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			tmp := make([]byte, len(v))
			copy(tmp, v)
			cache[string(k)] = tmp
		}

		return nil
	})
	db.Close()
	if err != nil {
		return nil, err
	}

	// Decode the jsons
	users := make([]User, 0, len(cache))
	for k, v := range cache {
		tmp := User{}
		err := json.Unmarshal(v, &tmp)
		if err != nil {
			continue
		}
		tmp.TelegramID, err = strconv.ParseInt(k, 10, 64)
		users = append(users, tmp)
	}

	return users, err
}

// Save the current User to Disk
func (u *User) Save() error {
	// Create the json
	rawJSON, err := json.Marshal(u)
	if err != nil {
		return err
	}

	// Open the database
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	// Start the transaction
	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Users"))
		err := b.Put([]byte(strconv.FormatInt(u.TelegramID, 10)), rawJSON)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}
