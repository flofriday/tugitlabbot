package main

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

const dbFile string = "data.db"

const (
	UserSetup int = iota
	UserNormal
)

type User struct {
	TelegramID  int64
	GitLabToken string
	LastChecked time.Time
	HasError    bool
	State       int
}

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

func LoadUser(telegramID int64) (User, error) {
	// Open the database
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return User{}, err
	}

	// Start a read transaction and close the database
	var rawJson []byte
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Users"))
		v := b.Get([]byte(strconv.FormatInt(telegramID, 10)))
		rawJson = make([]byte, len(v))
		copy(rawJson, v)
		return nil
	})
	db.Close()
	if err != nil {
		return User{}, err
	}

	// Decode the json
	user := User{}
	err = json.Unmarshal(rawJson, &user)
	if err != nil {
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

func (u *User) Save() error {
	// Create the json
	rawJson, err := json.Marshal(u)
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
		err := b.Put([]byte(strconv.FormatInt(u.TelegramID, 10)), rawJson)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}
