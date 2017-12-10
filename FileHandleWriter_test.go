package main

import (
	"bazil.org/fuse"
	"errors"
	"flag"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func init() {
	flag.StringVar(&stageDir, "stageDir", "/var/hdfs-mount", "set stage dir for test")
}

func TestWriteFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClock := &MockClock{}
	hdfsAccessor := NewMockHdfsAccessor(mockCtrl)
	fileName := "/testWriteFile_1"
	fs, _ := NewFileSystem(hdfsAccessor, "/tmp/x", []string{"*"}, false, false, NewDefaultRetryPolicy(mockClock), mockClock)

	hdfswriter := NewMockHdfsWriter(mockCtrl)
	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)

	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	root, _ := fs.Root()
	_, h, _ := root.(*Dir).Create(nil, &fuse.CreateRequest{Name: fileName, Mode: os.FileMode(0757)}, &fuse.CreateResponse{})

	// Test for newfilehandlewriter
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)
	writeHandle, err := NewFileHandleWriter(h.(*FileHandle), true)
	assert.Nil(t, err)

	// Test for normal write
	hdfsAccessor.EXPECT().StatFs().Return(FsInfo{capacity: uint64(100), used: uint64(20), remaining: uint64(80)}, nil)
	err = writeHandle.Write(h.(*FileHandle), nil, &fuse.WriteRequest{Data: []byte("hello world"), Offset: int64(11)}, &fuse.WriteResponse{})
	assert.Nil(t, err)
	assert.Equal(t, writeHandle.BytesWritten, uint64(11))

	hdfsAccessor.EXPECT().Remove("/testWriteFile_1").Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)
	binaryData := make([]byte, 65536, 65536)
	nr, _ := writeHandle.stagingFile.Read(binaryData)
	binaryData = binaryData[:nr]
	hdfswriter.EXPECT().Write(binaryData).Return(11, nil)
	err = writeHandle.Flush()
	assert.Nil(t, err)

	// Test for closing file
	err = writeHandle.Close()
	assert.Nil(t, err)

	// Test for writing file larger than available size
	hdfsAccessor.EXPECT().StatFs().Return(FsInfo{capacity: uint64(100), used: uint64(95), remaining: uint64(5)}, nil)
	err = writeHandle.Write(h.(*FileHandle), nil, &fuse.WriteRequest{Data: []byte("hello world"), Offset: int64(11)}, &fuse.WriteResponse{})
	assert.Equal(t, errors.New("Too large file"), err)
}

func TestFaultTolerantWriteFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockClock := &MockClock{}
	hdfsAccessor := NewMockHdfsAccessor(mockCtrl)
	fileName := "/testWriteFile_1"
	fs, _ := NewFileSystem(hdfsAccessor, "/tmp/x", []string{"*"}, false, false, NewDefaultRetryPolicy(mockClock), mockClock)

	hdfswriter := NewMockHdfsWriter(mockCtrl)
	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)

	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	root, _ := fs.Root()
	_, h, _ := root.(*Dir).Create(nil, &fuse.CreateRequest{Name: fileName, Mode: os.FileMode(0757)}, &fuse.CreateResponse{})

	// Test for newfilehandlewriter
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)
	writeHandle, err := NewFileHandleWriter(h.(*FileHandle), true)
	assert.Nil(t, err)

	// Test for normal write
	hdfsAccessor.EXPECT().StatFs().Return(FsInfo{capacity: uint64(100), used: uint64(20), remaining: uint64(80)}, nil)
	err = writeHandle.Write(h.(*FileHandle), nil, &fuse.WriteRequest{Data: []byte("hello world"), Offset: int64(11)}, &fuse.WriteResponse{})
	assert.Nil(t, err)
	assert.Equal(t, writeHandle.BytesWritten, uint64(11))

	hdfsAccessor.EXPECT().Remove("/testWriteFile_1").Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	// hdfswriter.EXPECT().Close().Return(nil)
	binaryData := make([]byte, 65536, 65536)
	nr, _ := writeHandle.stagingFile.Read(binaryData)
	binaryData = binaryData[:nr]

	// Mock the EOF error to test the fault tolerant write/flush
	hdfswriter.EXPECT().Write(binaryData).Return(0, io.EOF)
	hdfswriter.EXPECT().Close().Return(nil)
	err = writeHandle.FlushAttempt()
	assert.Equal(t, io.EOF, err)

	// The connection would be closed
	hdfsAccessor.EXPECT().Close().Return(nil)
	// New connection being established
	newhdfswriter := NewMockHdfsWriter(mockCtrl)
	hdfsAccessor.EXPECT().StatFs().Return(FsInfo{capacity: uint64(100), used: uint64(20), remaining: uint64(80)}, nil)
	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(newhdfswriter, nil)
	newbinaryData := make([]byte, 65536, 65536)
	newnr, _ := writeHandle.stagingFile.Read(binaryData)
	newbinaryData = newbinaryData[:newnr]
	newhdfswriter.EXPECT().Write(binaryData).Return(11, nil)
	newhdfswriter.EXPECT().Close().Return(nil)

	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	err = writeHandle.Flush()
	assert.Nil(t, err)

	// Test for closing file
	err = writeHandle.Close()
	assert.Nil(t, err)
}

func TestFlushFile(t *testing.T) {
	t.Skip("Cannot mock hdfsreader for overwiting file")

	mockCtrl := gomock.NewController(t)
	mockClock := &MockClock{}
	hdfsAccessor := NewMockHdfsAccessor(mockCtrl)
	fileName := "/testWriteFile_2"
	fs, _ := NewFileSystem(hdfsAccessor, "/tmp/x", []string{"*"}, false, false, NewDefaultRetryPolicy(mockClock), mockClock)

	hdfswriter := NewMockHdfsWriter(mockCtrl)
	hdfsAccessor.EXPECT().Remove(fileName).Return(nil)
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)

	root, _ := fs.Root()
	_, h, _ := root.(*Dir).Create(nil, &fuse.CreateRequest{Name: fileName, Mode: os.FileMode(0757)}, &fuse.CreateResponse{})

	// Test for newfilehandlewriter with existing file
	hdfsAccessor.EXPECT().CreateFile(fileName, os.FileMode(0757)).Return(hdfswriter, nil)
	hdfswriter.EXPECT().Close().Return(nil)
	hdfsAccessor.EXPECT().StatFs().Return(FsInfo{capacity: uint64(100), used: uint64(20), remaining: uint64(80)}, nil)
	hdfsAccessor.EXPECT().Stat("/testWriteFile_2").Return(Attrs{Name: "testWriteFile_2"}, nil)
	// BUG: cannot mock the returned hdfsreader here
	hdfsAccessor.EXPECT().OpenRead("/testWriteFile_2").Return(nil, nil)
	writeHandle, err := NewFileHandleWriter(h.(*FileHandle), false)
	assert.Nil(t, err)

	// Test for flush
	_ = writeHandle.Write(h.(*FileHandle), nil, &fuse.WriteRequest{Data: []byte("hello world"), Offset: int64(11)}, &fuse.WriteResponse{})
	err = writeHandle.Flush()
	assert.Nil(t, err)

	err = writeHandle.Close()
	assert.Nil(t, err)
}
