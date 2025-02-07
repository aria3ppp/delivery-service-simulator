package config

type Config struct {
	WorkerConfig WorkerConfig
}

type WorkerConfig struct {
	PendingIntervalInSeconds int64
	PendingWorkerBatchSize   int
	ShipmentWorkerBatchSize  int
}
