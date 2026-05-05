package cmd

import (
	pb "pb"
)

func getChanFileToDisk(pbIn *pb.File) *pb.File {
	success, err := pbFileChunkSave(pbIn)

	resp := &pb.File{}
	if success == false && err == nil {
		resp.Status = 206
	}

	if success == true && err == nil {
		resp.Status = 201
	}

	if err != nil {
		resp.Status = 500
	}

	return resp
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
