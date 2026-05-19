package form

import "testing"

func TestFileResourceListRejectsInvalidIsPublic(t *testing.T) {
	err := bindJSONBody(t, `{"is_public":2}`, NewFileResourceListQuery())
	if err == nil {
		t.Fatal("expected invalid is_public to fail validation")
	}
}

func TestFileResourceListRejectsInvalidFileType(t *testing.T) {
	err := bindJSONBody(t, `{"file_type":"exe"}`, NewFileResourceListQuery())
	if err == nil {
		t.Fatal("expected invalid file_type to fail validation")
	}
}

func TestFileResourceListAllowsMultiFileTypeAndOther(t *testing.T) {
	err := bindJSONBody(t, `{"file_type":"pdf, word,other"}`, NewFileResourceListQuery())
	if err != nil {
		t.Fatalf("expected multi file_type and other to pass validation, got %v", err)
	}
}

func TestFileUploadCompleteRejectsMultiFileType(t *testing.T) {
	err := bindJSONBody(t, `{"origin_name":"a.txt","storage_driver":"s3","bucket":"assets","object_key":"a.txt","file_type":"pdf,word"}`, NewFileUploadCompleteForm())
	if err == nil {
		t.Fatal("expected upload complete multi file_type to fail validation")
	}
}

func TestFileMultipartCompleteRejectsMultiFileType(t *testing.T) {
	err := bindJSONBody(t, `{"origin_name":"a.txt","driver":"s3","bucket":"assets","object_key":"a.txt","file_type":"pdf,word","parts":[{"part_number":1,"etag":"etag"}]}`, NewFileMultipartCompleteForm())
	if err == nil {
		t.Fatal("expected multipart complete multi file_type to fail validation")
	}
}

func TestFileMultipartCompleteAllowsReuseWithoutUploadIDAndParts(t *testing.T) {
	err := bindJSONBody(t, `{"reuse":true,"file_object_id":1,"origin_name":"a.txt","file_type":"pdf"}`, NewFileMultipartCompleteForm())
	if err != nil {
		t.Fatalf("expected reuse multipart complete without upload_id, bucket, object_key and parts to pass validation, got %v", err)
	}
}

func TestFileResourceIDRejectsZero(t *testing.T) {
	err := bindJSONBody(t, `{"id":0}`, NewFileResourceIDForm())
	if err == nil {
		t.Fatal("expected zero id to fail validation")
	}
}

func TestFileResourceListRejectsInvalidStorageDriver(t *testing.T) {
	err := bindJSONBody(t, `{"storage_driver":"ftp"}`, NewFileResourceListQuery())
	if err == nil {
		t.Fatal("expected invalid storage_driver to fail validation")
	}
}

func TestFileResourceListAllowsStorageFilters(t *testing.T) {
	err := bindJSONBody(t, `{"storage_driver":"s3","storage_status":"stored","is_referenced":1,"is_deleted":0}`, NewFileResourceListQuery())
	if err != nil {
		t.Fatalf("expected storage filters to pass validation, got %v", err)
	}
}

func TestFileReferenceListAllowsIDAlias(t *testing.T) {
	err := bindJSONBody(t, `{"id":1}`, NewFileReferenceListQuery())
	if err != nil {
		t.Fatalf("expected id alias to pass validation, got %v", err)
	}
}

func TestFileMoveRejectsZeroID(t *testing.T) {
	err := bindJSONBody(t, `{"ids":[1,0],"folder_id":2}`, NewFileMoveForm())
	if err == nil {
		t.Fatal("expected zero file id to fail validation")
	}
}

func TestFileUploadCompleteRejectsLocalDriver(t *testing.T) {
	err := bindJSONBody(t, `{"origin_name":"a.txt","storage_driver":"local","object_key":"a.txt"}`, NewFileUploadCompleteForm())
	if err == nil {
		t.Fatal("expected local direct complete to fail validation")
	}
}

func TestFileResourceListAllowsFolderFilters(t *testing.T) {
	err := bindJSONBody(t, `{"folder_id":1,"include_subfolder":1}`, NewFileResourceListQuery())
	if err != nil {
		t.Fatalf("expected folder filters to pass validation, got %v", err)
	}
}
