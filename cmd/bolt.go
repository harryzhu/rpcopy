package cmd

import (
	"context"
	"os"
	"path/filepath"
	pb "pb"
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

	filelist2 := filelist

	t1 := time.Now()
	db.Batch(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("pbfiles"))
		for _, relPath := range filelist2 {
			fkey := strings.TrimPrefix(relPath, SourceDir)
			fpath := filepath.Join(SourceDir, relPath)
			//DebugInfo("createBolt: PUT", fpath)
			finfo, err := os.Stat(fpath)
			if err != nil {
				PrintError("createBolt:os.Stat", err)
				continue
			}
			bfinfo := finfo2FileInfoLite(finfo)
			bdata, err := os.ReadFile(fpath)
			if err != nil {
				PrintError("createBolt:os.ReadFile", err)
				continue
			}
			keyInfo := strings.Join([]string{"INFO", fkey}, "/")
			keyData := strings.Join([]string{"DATA", fkey}, "/")

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
	PrintlnInfo("green", "createBolt: Elapse:", time.Since(t1))

	db.Close()

	finfo, err := os.Stat(dbPath)
	if err != nil {
		PrintError("createBolt:os.Stat", err)
		return err
	}

	DebugInfo("createBolt:os.Stat", finfo.Size())

	pbMisc := &pb.Misc{Mtype: "pbBolt", Data: nil}
	pbFileData, err := os.ReadFile(dbPath)
	if err != nil {
		PrintError("createBolt:os.ReadFile", err)
		return err
	}
	pbMisc.Data = pbFileData
	DebugInfo("createBolt:fsize", len(pbMisc.Data))

	_, err = gClient.PutMisc(context.Background(), pbMisc)
	if err != nil {
		PrintError("createBolt:pbFileChunkSend", err)
		return err
	}
	time.Sleep(time.Second)
	if FileExists(dbPath) {
		err := os.Remove(dbPath)
		PrintError("createBolt:os.Remove", err)
		return err
	}

	return nil
}
