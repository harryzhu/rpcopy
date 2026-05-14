package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

func createBolt(filelist []string, dbName string) (err error) {
	dbPath, _ := filepath.Abs(filepath.Join(LogDir, hashString([]byte(dbName))))
	if FileExists(dbPath) {
		err := os.Remove(dbPath)
		PrintError("createBolt:createBolt", err)
		return err
	}

	DebugInfo("createBolt", dbPath)
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		PrintError("createBolt:Open", err)
		return err
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("pbfiles"))
		if err != nil {
			PrintError("createBolt:CreateBucketIfNotExists", err)
			return err
		}
		return nil
	})

	t1 := time.Now()
	var fkey, fpath, keyInfo, keyData string
	var finfo os.FileInfo
	var bfinfo, bdata []byte
	db.Batch(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("pbfiles"))
		for _, relPath := range filelist {
			fkey = strings.TrimPrefix(relPath, SourceDir)
			fpath = filepath.Join(SourceDir, relPath)
			//DebugInfo("createBolt: PUT", fpath)
			finfo, err = os.Stat(fpath)
			if err != nil {
				PrintError("createBolt:os.Stat", err)
				continue
			}
			bfinfo = finfo2FileInfoLite(finfo)
			bdata, err = os.ReadFile(fpath)
			if err != nil {
				PrintError("createBolt:os.ReadFile", err)
				continue
			}
			keyInfo = strings.Join([]string{"INFO", fkey}, "/")
			keyData = strings.Join([]string{"DATA", fkey}, "/")

			err = bkt.Put([]byte(keyInfo), ZstdBytes(bfinfo))
			if err != nil {
				PrintError("createBolt:info:Put", err)
				continue
			}

			err = bkt.Put([]byte(keyData), ZstdBytes(bdata))
			if err != nil {
				PrintError("createBolt:data:Put", err)
			}

		}
		return nil
	})

	db.Close()

	finfo, err = os.Stat(dbPath)
	if err != nil {
		PrintError("createBolt:os.Stat", err)
		return err
	}

	PrintlnInfo("green", "createBolt: Elapse", time.Since(t1), ", Size: ", finfo.Size()>>20, "MB")

	pbFile := file2pbFile(dbPath, finfo, "bolt")

	t1 = GetNowTime()
	err = pbBoltSend(dbPath, pbFile)
	if err != nil {
		PrintError("createBolt:pbFileChunkSend", err)
		return err
	}
	PrintlnInfo("green", "sendBolt: Elapse", time.Since(t1))

	//time.Sleep(time.Second)
	if FileExists(dbPath) {
		err := os.Remove(dbPath)
		PrintError("createBolt:os.Remove", err)
		return err
	}

	return nil
}
