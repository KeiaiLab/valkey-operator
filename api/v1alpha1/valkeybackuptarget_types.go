/*
Copyright 2026 Keiailab.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupTargetType — 외부 저장 종류. 첫 구현은 S3 (MinIO/Ceph RGW 호환).
// GCS / Azure 는 추후 type 분기 + 별도 sub-spec 추가 (ADR-0016).
//
// +kubebuilder:validation:Enum=S3;GCS;Azure
type BackupTargetType string

const (
	BackupTargetTypeS3    BackupTargetType = "S3"
	BackupTargetTypeGCS   BackupTargetType = "GCS"
	BackupTargetTypeAzure BackupTargetType = "Azure"
)

// S3Spec — S3-compatible 외부 저장 정의 (AWS S3, MinIO, Ceph RGW 등).
type S3Spec struct {
	// S3 endpoint URL. AWS 표준: https://s3.<region>.amazonaws.com
	// MinIO 사내: https://minio.local:9000 등.
	Endpoint string `json:"endpoint"`

	// 리전 (AWS 필수, MinIO 도 1개 이상 명시 — e.g. "us-east-1").
	Region string `json:"region"`

	// 버킷 이름. 사전 생성 필요 (operator 가 자동 생성하지 않음).
	Bucket string `json:"bucket"`

	// object key prefix. 예: "cluster-A/" → cluster-A/<timestamp>/dump.rdb
	//
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// path-style URL 강제. MinIO / Ceph RGW 시 true.
	//
	// +kubebuilder:default=false
	// +optional
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`

	// 자격증명 Secret 참조 (access key + secret key).
	CredentialsSecretRef S3CredentialsSecretRef `json:"credentialsSecretRef"`

	// 서버측 암호화. AWS S3 의 "AES256" / "aws:kms" / 미명시 (provider 기본값).
	//
	// +optional
	ServerSideEncryption string `json:"serverSideEncryption,omitempty"`
}

// S3CredentialsSecretRef — Secret 안 의 access/secret key 매핑.
//
// 자격증명 회전 시 Job 영향 차단: backup Job 은 spawn 시점 envFrom Secret
// snapshot 사용 — rotation 후 새 Job 만 새 자격증명. ADR-0016.
type S3CredentialsSecretRef struct {
	// Secret 이름. 같은 namespace 의 Secret 만 허용.
	Name string `json:"name"`

	// access key ID 가 들어 있는 key 이름.
	//
	// +kubebuilder:default="AWS_ACCESS_KEY_ID"
	// +optional
	AccessKeyIDKey string `json:"accessKeyIDKey,omitempty"`

	// secret access key 가 들어 있는 key 이름.
	//
	// +kubebuilder:default="AWS_SECRET_ACCESS_KEY"
	// +optional
	SecretAccessKeyKey string `json:"secretAccessKeyKey,omitempty"`
}

// GCSSpec — Google Cloud Storage 외부 저장 정의 (cloud.google.com/go/storage v1.62.1).
//
// 자격증명: service account JSON key 를 Secret 에 저장. backup Job 의 환경변수
// GOOGLE_APPLICATION_CREDENTIALS 가 가리키는 파일에 mount.
type GCSSpec struct {
	// 버킷 이름. 사전 생성 필요.
	Bucket string `json:"bucket"`

	// object key prefix. 예: "cluster-A/".
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// 자격증명 Secret 참조 (service account JSON).
	CredentialsSecretRef GCSCredentialsSecretRef `json:"credentialsSecretRef"`
}

// GCSCredentialsSecretRef — service account JSON 이 들어 있는 Secret.
type GCSCredentialsSecretRef struct {
	// Secret 이름.
	Name string `json:"name"`

	// service account JSON 이 들어 있는 key 이름. 기본 "key.json".
	// +kubebuilder:default="key.json"
	// +optional
	ServiceAccountJSONKey string `json:"serviceAccountJSONKey,omitempty"`
}

// AzureSpec — Azure Blob Storage 외부 저장 정의
// (github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.x).
type AzureSpec struct {
	// storage account 이름. e.g. "mystorageacct".
	AccountName string `json:"accountName"`

	// container 이름 (S3 의 bucket 등가). 사전 생성 필요.
	Container string `json:"container"`

	// blob name prefix. 예: "cluster-A/".
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// service URL override. 기본: https://<accountName>.blob.core.windows.net.
	// Azure China / Government cloud / Azurite (test) 용.
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// 자격증명 Secret 참조 (account key 또는 SAS token).
	CredentialsSecretRef AzureCredentialsSecretRef `json:"credentialsSecretRef"`
}

// AzureCredentialsSecretRef — Azure storage account key 또는 SAS.
type AzureCredentialsSecretRef struct {
	// Secret 이름.
	Name string `json:"name"`

	// account key 가 들어 있는 key 이름. 기본 "AZURE_STORAGE_ACCOUNT_KEY".
	// +kubebuilder:default="AZURE_STORAGE_ACCOUNT_KEY"
	// +optional
	AccountKeyKey string `json:"accountKeyKey,omitempty"`
}

// ValkeyBackupTargetSpec — 외부 저장 target 정의.
//
// ValkeyBackup.Spec.Destination.TargetRef + ValkeyRestore.Spec.Source.TargetRef
// 가 본 CR 을 참조 (대칭성, ADR-0016).
type ValkeyBackupTargetSpec struct {
	// +kubebuilder:default=S3
	// +optional
	Type BackupTargetType `json:"type,omitempty"`

	// Type=S3 시 필수.
	// +optional
	S3 *S3Spec `json:"s3,omitempty"`

	// Type=GCS 시 필수.
	// +optional
	GCS *GCSSpec `json:"gcs,omitempty"`

	// Type=Azure 시 필수.
	// +optional
	Azure *AzureSpec `json:"azure,omitempty"`
}

// BackupTargetPhase — reachability 라이프사이클.
//
// +kubebuilder:validation:Enum=Pending;Reachable;Unreachable
type BackupTargetPhase string

const (
	BackupTargetPhasePending     BackupTargetPhase = "Pending"
	BackupTargetPhaseReachable   BackupTargetPhase = "Reachable"
	BackupTargetPhaseUnreachable BackupTargetPhase = "Unreachable"
)

// ValkeyBackupTargetStatus — reachability + 마지막 검증 시점.
type ValkeyBackupTargetStatus struct {
	Phase BackupTargetPhase `json:"phase,omitempty"`

	// 마지막 reachability 검증 성공 시각.
	LastVerifiedAt *metav1.Time `json:"lastVerifiedAt,omitempty"`

	Message            string `json:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vbt
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="LastVerified",type="date",JSONPath=".status.lastVerifiedAt"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ValkeyBackupTarget — 외부 저장 (S3/GCS/Azure) endpoint + 자격증명 추상화.
// ValkeyBackup ↔ ValkeyRestore 가 동일 target 참조 (ADR-0016).
type ValkeyBackupTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ValkeyBackupTargetSpec   `json:"spec,omitempty"`
	Status ValkeyBackupTargetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type ValkeyBackupTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ValkeyBackupTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ValkeyBackupTarget{}, &ValkeyBackupTargetList{})
}

func (t *ValkeyBackupTarget) GetConditions() *[]metav1.Condition { return &t.Status.Conditions }
