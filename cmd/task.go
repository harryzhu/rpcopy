package cmd

import (
	pb "pb"
)

func getChanFileToDisk(pbIn *pb.File) error {
	statusCode, err := pbFileChunkSave(pbIn)

	pbSaveStatus[string(pbIn.Path)] = statusCode

	if err != nil {
		pbSaveStatus[string(pbIn.Path)] = 500
	}

	return nil
}

func taskChanFile() error {
	for {
		ele := <-chanFile
		if ele.Status == -1 {
			DebugInfo("_COPYSTATUS:chanFile", "ALL_DONE")
		}
		DebugInfo("_COPYSTATUS:chanFile", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile1() error {
	for {
		ele := <-chanFile1
		if ele.Status == -1 {
			DebugInfo("_COPYSTATUS:chanFile1", "ALL_DONE")
		}
		DebugInfo("_COPYSTATUS:chanFile1", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile2() error {
	for {
		ele := <-chanFile2
		if ele.Status == -1 {
			DebugInfo("_COPYSTATUS:chanFile2", "ALL_DONE")
		}
		DebugInfo("_COPYSTATUS:chanFile2", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}

func taskChanFile3() error {
	for {
		ele := <-chanFile3
		if ele.Status == -1 {
			DebugInfo("_COPYSTATUS:chanFile3", "ALL_DONE")
		}
		DebugInfo("_COPYSTATUS:chanFile3", ele.ChanNum, ": ", string(ele.Path))

		getChanFileToDisk(ele)

	}

	return nil
}
