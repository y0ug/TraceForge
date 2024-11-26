package sbapi

import (
	"TraceForge/internals/commons"
	"TraceForge/internals/mq"
	"database/sql"
	"encoding/json"
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
	MQClient     *mq.Client
}

type DB struct {
	DB *sql.DB
}

type Config struct {
	AuthToken    string
	S3BucketName string
	S3Region     string
	S3Endpoint   string
	S3AccessKey  string
	S3SecretKey  string
	MqURL        string
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
	ID          string            `toml:"agent_uuid"`
	Name        string            `toml:"name"`
	Provider    string            `toml:"provider,omitempty"`
	Plugins     []string          `toml:"plugins,omitempty"`
	HvapiName   string            `toml:"hvapi_name,omitempty"`
	HvapiConfig HvapiAgentsConfig `toml:"-"`
}

type AnalysisTask struct {
	ID        uuid.UUID       `json:"id"`
	FileID    uuid.UUID       `json:"file_id"`
	AgentID   uuid.UUID       `json:"agent_id"`
	Plugin    string          `json:"plugin"`
	Status    string          `json:"status"`
	Args      json.RawMessage `json:"args,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AgentInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Plugins []string `json:"plugins,omitempty"`
}
