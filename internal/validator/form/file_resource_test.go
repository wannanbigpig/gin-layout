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
	err := bindJSONBody(t, `{"complete_token":"token","origin_name":"a.txt","size":12,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mime_type":"text/plain","is_public":1,"storage_driver":"aliyun_oss","bucket":"assets","object_key":"a.txt","file_type":"pdf,word"}`, NewFileUploadCompleteForm())
	if err == nil {
		t.Fatal("expected upload complete multi file_type to fail validation")
	}
}

func TestFileUploadCredentialBatchAllowsSharedDriverAndItems(t *testing.T) {
	err := bindJSONBody(t, `{"driver":"aliyun_oss","is_public":1,"upload_scene":"picker","items":[{"client_id":"c1","folder_id":2,"origin_name":"a.txt","size":12,"mime_type":"text/plain","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`, NewFileUploadCredentialBatchForm())
	if err != nil {
		t.Fatalf("expected credential batch to pass validation, got %v", err)
	}
}

func TestFileUploadCredentialRequiresHash(t *testing.T) {
	err := bindJSONBody(t, `{"origin_name":"a.txt","size":12,"mime_type":"text/plain","driver":"aliyun_oss"}`, NewFileUploadCredentialForm())
	if err == nil {
		t.Fatal("expected credential without hash to fail validation")
	}
}

func TestFileUploadCredentialBatchRequiresClientIDAndHash(t *testing.T) {
	err := bindJSONBody(t, `{"items":[{"origin_name":"a.txt","size":12,"hash":"short"}]}`, NewFileUploadCredentialBatchForm())
	if err == nil {
		t.Fatal("expected invalid credential batch item to fail validation")
	}
}

func TestFileUploadCompleteBatchRejectsLocalDriver(t *testing.T) {
	err := bindJSONBody(t, `{"driver":"local","items":[{"client_id":"c1","complete_token":"token","origin_name":"a.txt","size":12,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mime_type":"text/plain"}]}`, NewFileUploadCompleteBatchForm())
	if err == nil {
		t.Fatal("expected local complete batch driver to fail validation")
	}
}

func TestFileUploadCompleteBatchAllowsReuseItem(t *testing.T) {
	err := bindJSONBody(t, `{"driver":"aliyun_oss","items":[{"client_id":"c1","complete_token":"token","reuse":true,"file_object_id":1,"origin_name":"a.txt","size":12,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mime_type":"text/plain"}]}`, NewFileUploadCompleteBatchForm())
	if err != nil {
		t.Fatalf("expected complete batch reuse item to pass validation, got %v", err)
	}
}

func TestFileMultipartCompleteRejectsMultiFileType(t *testing.T) {
	err := bindJSONBody(t, `{"complete_token":"token","origin_name":"a.txt","size":12,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mime_type":"text/plain","driver":"aliyun_oss","bucket":"assets","object_key":"a.txt","file_type":"pdf,word","parts":[{"part_number":1,"etag":"etag"}]}`, NewFileMultipartCompleteForm())
	if err == nil {
		t.Fatal("expected multipart complete multi file_type to fail validation")
	}
}

func TestFileMultipartCompleteAllowsReuseTokenWithoutUploadIDAndParts(t *testing.T) {
	err := bindJSONBody(t, `{"complete_token":"token","reuse":true,"file_object_id":1,"origin_name":"a.txt","size":12,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","mime_type":"application/pdf","file_type":"pdf"}`, NewFileMultipartCompleteForm())
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

func TestFileFolderDeleteRequiresConfirm(t *testing.T) {
	err := bindJSONBody(t, `{"id":1}`, NewFileFolderDeleteForm())
	if err == nil {
		t.Fatal("expected folder delete without confirm to fail validation")
	}
}

func TestFileFolderDeleteAllowsExplicitConfirm(t *testing.T) {
	err := bindJSONBody(t, `{"id":1,"confirm":1}`, NewFileFolderDeleteForm())
	if err != nil {
		t.Fatalf("expected folder delete with confirm to pass validation, got %v", err)
	}
}

func TestFileFolderStatsDoesNotRequireDeleteConfirm(t *testing.T) {
	err := bindJSONBody(t, `{"id":1}`, NewFileFolderStatsForm())
	if err != nil {
		t.Fatalf("expected folder stats to pass without delete confirm, got %v", err)
	}
}

func TestFileResourceListRejectsInvalidStorageDriver(t *testing.T) {
	err := bindJSONBody(t, `{"storage_driver":"ftp"}`, NewFileResourceListQuery())
	if err == nil {
		t.Fatal("expected invalid storage_driver to fail validation")
	}
}

func TestFileResourceListAllowsStorageFilters(t *testing.T) {
	err := bindJSONBody(t, `{"storage_driver":"aliyun_oss","storage_status":"stored","is_referenced":1,"is_deleted":0}`, NewFileResourceListQuery())
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
