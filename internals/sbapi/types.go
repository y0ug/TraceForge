package sbapi

import (
	"TraceForge/internals/commons"
	"database/sql"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type Server struct {
	*commons.Server
	Config       Config
	S3Client     *s3.Client
	DB           *DB
	RedisClient  *redis.Client
	TaskManager  *TaskManager
	AgentsConfig *AgentsConfig
}

type DB struct {
	DB *sql.DB
}

type Config struct {
	AuthToken      string
	HvApiAuthToken string
	HvApiUrl       string
	S3BucketName   string
	S3Region       string
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
}

type UploadResponse struct {
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`    // UUID
	Key       string `json:"key"`        // S3 key
	ExpiresIn int64  `json:"expires_in"` // Expiration in seconds
}

type FileInfo struct {
	ID        uuid.UUID `json:"id"`
	S3Key     string    `json:"s3_key"`
	Filename  string    `json:"filename,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Sha1      string    `json:"sha1,omitempty"`
	Sha256    string    `json:"sha256,omitempty"`
}

type AgentsConfig struct {
	Hvapi         map[string]HvapiAgentsConfig `toml:"hvapi"`
	AgentDefaults AgentDefaultsConfig          `toml:"agent_defaults,omitempty"`
	Agents        []AgentConfig                `toml:"agent"`
}

type AgentDefaultsConfig struct {
	Plugins   []string `toml:"plugins,omitempty"`
	HvapiName string   `toml:"hvapi_name,omitempty"`
	Provider  string   `toml:"provider,omitempty"`
}

type HvapiAgentsConfig struct {
	URL       string `toml:"url"`
	AuthToken string `toml:"auth_token"`
}

type AgentConfig struct {
	Name      string   `toml:"name"`
	AgentUUID string   `toml:"agent_uuid"`
	Provider  string   `toml:"provider,omitempty"`
	Plugins   []string `toml:"plugins,omitempty"`
	HvapiName string   `toml:"hvapi_name,omitempty"`
}
