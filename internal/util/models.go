package util

// RestAPIConfig is the top-level structure of the kkon config file.
type RestAPIConfig struct {
	KafkaConnect KafkaConnectConfig `yaml:"kafkaConnect"`
}

// KafkaConnectConfig holds the Kafka Connect REST API connection settings.
type KafkaConnectConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
