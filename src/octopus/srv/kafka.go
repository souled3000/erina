package srv

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
)

var (
	certFile  string = ""
	keyFile          = ""
	caFile           = ""
	verifySsl bool   = false
)

type Server struct {
	Producer   sarama.SyncProducer
	Consumer   sarama.Consumer
	ProducerCh chan KafkaMsg
	ConsumerCh chan KafkaMsg
}
type KafkaMsg struct {
	Msg   []byte
	Topic string
	Key   int64
}

func KafkaConsumer(brokerList []string, topics string) *Server {
	server := &Server{
		Consumer:   newConsumer(brokerList),
		ConsumerCh: make(chan KafkaMsg, 10000),
	}
	go server.consume(topics)
	return server
}
func KafkaProducer(brokerList []string) *Server {
	server := &Server{
		Producer:   newProducer(brokerList),
		ProducerCh: make(chan KafkaMsg, 10000),
	}
	go server.produce()
	return server

}
func (s *Server) produce() {
	for {
		msg := <-s.ProducerCh
		partition, offset, err := s.Producer.SendMessage(&sarama.ProducerMessage{
			Topic: msg.Topic,
			Value: sarama.StringEncoder(msg.Msg),
			Key:   sarama.StringEncoder(msg.Key),
		})
		if err != nil {
			glog.Errorf("[Kafka]SendToKafka Error: %v,data[%x],Topic [%s],Position [%v,%v],Key [%v]", err.Error(), msg.Msg, msg.Topic, partition, offset, msg.Key)
		}
	}
}

func (s *Server) consume(consumerTopics string) {
	topics := strings.Split(consumerTopics, ",")
	for _, topic := range topics {
		partitions, _ := s.Consumer.Partitions(topic)
		s.Consumer.Topics()
		for partition := range partitions {
			pc, _ := s.Consumer.ConsumePartition(topic, int32(partition), sarama.OffsetNewest)
			go func() {
				signals := make(chan os.Signal, 1)
				signal.Notify(signals, os.Kill, os.Interrupt)
				for {
					select {
					case <-signals:
						pc.AsyncClose()
					case msg := <-pc.Messages():
						km := KafkaMsg{msg.Value, msg.Topic, 0}
						s.ConsumerCh <- km
					}

				}
			}()

		}

	}
}

func createTlsConfiguration() (t *tls.Config) {
	if certFile != "" && keyFile != "" && caFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			glog.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			glog.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		t = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: verifySsl,
		}
	}
	// will be nil by default if nothing is provided
	return t
}

func (s *Server) Close() error {
	if err := s.Producer.Close(); err != nil {
		glog.Info("Failed to shut down data collector cleanly", err)
	}

	if err := s.Consumer.Close(); err != nil {
		glog.Info("Failed to shut down Consumer cleanly", err)
	}

	return nil
}

func newConsumer(brokerList []string) sarama.Consumer {
	config := sarama.NewConfig()
	tlsConfig := createTlsConfiguration()
	if tlsConfig != nil {
		config.Net.TLS.Config = tlsConfig
		config.Net.TLS.Enable = true
	}

	consumer, err := sarama.NewConsumer(brokerList, nil)
	if err != nil {
		panic(err)
	}
	return consumer
}

func newProducer(brokerList []string) sarama.SyncProducer {

	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	tlsConfig := createTlsConfiguration()
	if tlsConfig != nil {
		config.Net.TLS.Config = tlsConfig
		config.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.

	producer, err := sarama.NewSyncProducer(brokerList, config)
	if err != nil {
		glog.Info("Failed to start Sarama producer:", err)
	}

	return producer
}
