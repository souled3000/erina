package kafka

import (
	"github.com/Shopify/sarama"
	"octopus/msgs"

	"crypto/tls"
	"crypto/x509"
	"github.com/golang/glog"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
)

var (
	certFile  string = ""
	keyFile          = ""
	caFile           = ""
	verifySsl bool   = false
)

func Serve(brokerList []string, sendTopicName, reciveTopicName string) *Server {
	server := &Server{
		DataCollector: newDataCollector(brokerList),
		Consumer:      newDataConsumer(brokerList),
		ProducerCh:    make(chan []byte, 10000),
		ConsumerCh:    make(chan []byte, 10000),
	}
	if sendTopicName != "" {
		go server.SendToKafka(sendTopicName)
	}
	if reciveTopicName != "" {
		go server.ReciveFromKafka(reciveTopicName)
	}

	return server
}

func (s *Server) SendToKafka(sendTopicName string) {
	for {
		msg := <-s.ProducerCh
		m := &msgs.Msg{}
		m.Final = msg
		m.Binary2Msg()
		partition, offset, err := s.DataCollector.SendMessage(&sarama.ProducerMessage{
			Topic: sendTopicName,
			Value: sarama.StringEncoder(msg),
			Key:   sarama.StringEncoder(m.FHSrcId),
		})
		if err != nil {
			glog.Errorf("[Kafka]SendToKafka Error: %v,data[%x],Topic [%s],Position [%v,%v],Key [%s]", err.Error(), msg, sendTopicName, partition, offset, m.FHSrcMac)
		} else {
			if glog.V(5) {
				glog.Infof("[Kafka]SendToKafka Success,data[%x],Topic [%s],Position [%v,%v],Key [%s]", msg, sendTopicName, partition, offset, m.FHSrcMac)
			}
		}
	}
}

func (s *Server) ReciveFromKafka(reciveTopicName string) {
	topic := reciveTopicName
	for {
		partitions, _ := s.Consumer.Partitions(topic)
		for partition := range partitions {
			pc, _ := s.Consumer.ConsumePartition(topic, int32(partition), sarama.OffsetNewest)
			go func() {
				signals := make(chan os.Signal, 1)
				signal.Notify(signals, os.Kill, os.Interrupt)
				<-signals
				pc.AsyncClose()
			}()
			for msg := range pc.Messages() {
				s.ConsumerCh <- msg.Value
				if glog.V(5) {
					glog.Infof("[Kafka]ReciveFromKafka Success,data [%x],Topic [%s]", msg.Value, reciveTopicName)
				}
			}

		}

	}
}

func createTlsConfiguration() (t *tls.Config) {
	if certFile != "" && keyFile != "" && caFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			log.Fatal(err)
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

type Server struct {
	DataCollector sarama.SyncProducer
	Consumer      sarama.Consumer
	ProducerCh    chan []byte
	ConsumerCh    chan []byte
}

func (s *Server) Close() error {
	if err := s.DataCollector.Close(); err != nil {
		log.Println("Failed to shut down data collector cleanly", err)
	}

	if err := s.Consumer.Close(); err != nil {
		log.Println("Failed to shut down Consumer cleanly", err)
	}

	return nil
}

func newDataConsumer(brokerList []string) sarama.Consumer {
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

func newDataCollector(brokerList []string) sarama.SyncProducer {

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
		log.Fatalln("Failed to start Sarama producer:", err)
	}

	return producer
}
