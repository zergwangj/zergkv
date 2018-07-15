// Copyright © 2018 zergwangj zergwangj@163.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"fmt"
	"github.com/boltdb/bolt"
	"path/filepath"
)

const (
	dbFile    = "kv.db"
	kvsBucket = "kvsBucket"
)

type Kv struct {
	Dir string
	Db  *bolt.DB
}

func NewKv(dir string) (*Kv, error) {
	db, err := bolt.Open(filepath.Join(dir, dbFile), 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kvsBucket))
		if b == nil {
			fmt.Println("No existing kv bucket found. Creating a new one...")
			_, err := tx.CreateBucket([]byte(kvsBucket))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	k := &Kv{
		Dir: dir,
		Db:  db,
	}
	return k, nil
}

func (k *Kv) Close() {
	k.Db.Close()
}

func (k *Kv) Get(key []byte) ([]byte, error) {
	var value []byte
	var err error

	err = k.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kvsBucket))
		if b != nil {
			value = b.Get([]byte(key))
			if value == nil {
				return fmt.Errorf("key not found")
			}
			return nil
		}
		return fmt.Errorf("Bucket kv is not found")
	})
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (k *Kv) Set(key []byte, value []byte) error {
	err := k.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kvsBucket))
		if b != nil {
			err := b.Put([]byte(key), value)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (k *Kv) Delete(key []byte) error {
	err := k.Db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kvsBucket))
		if b != nil {
			data := b.Get([]byte(key))
			if data == nil {
				return fmt.Errorf("key not found")
			}
			return nil
		}
		return fmt.Errorf("Bucket kv is not found")
	})
	if err != nil {
		return err
	}

	err = k.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(kvsBucket))
		if b != nil {
			err = b.Delete([]byte(key))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
