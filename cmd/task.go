package cmd

import (
	pb "pb"
)

func getChanFileToDisk(pbIn *pb.File) error {
	if isPathValid(string(pbIn.Path)) == false {
		err := NewError("path invalid: ", string(pbIn.Path))
		PrintError("getChanFileToDisk", err)

		safePbSaveStatus.Store(string(pbIn.Path), int64(412))
		return err
	}

	statusCode, err := pbFileChunkSave(pbIn)

	if err != nil {
		PrintError("getChanFileToDisk", err)
		safePbSaveStatus.Store(string(pbIn.Path), int64(500))
	} else {
		safePbSaveStatus.Store(string(pbIn.Path), int64(statusCode))
	}

	return nil
}

func taskChanFile() error {
	for {
		ele := <-chanFile
		DebugInfo("taskChanFile", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile1() error {
	for {
		ele := <-chanFile1
		DebugInfo("taskChanFile1", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile2() error {
	for {
		ele := <-chanFile2
		DebugInfo("taskChanFile2", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile3() error {
	for {
		ele := <-chanFile3
		DebugInfo("taskChanFile3", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}
