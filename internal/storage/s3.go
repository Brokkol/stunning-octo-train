package storage

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client *s3.Client
	bucket string
}

func NewS3Client() (*S3Client, error) {
	// Логин и пароль (AWS_ACCESS_KEY_ID и AWS_SECRET_ACCESS_KEY) SDK сам берёт из переменных окружения
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(os.Getenv("S3_REGION")))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// Говорим SDK слать запросы не в Amazon, а в наш контейнер SeaweedFS
		o.BaseEndpoint = aws.String(os.Getenv("S3_ENDPOINT"))
		o.UsePathStyle = true
		// Новые версии SDK добавляют контрольные суммы, которые SeaweedFS может не понять
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	s := &S3Client{client: client, bucket: os.Getenv("S3_BUCKET")}

	// Создаём бакет. Если он уже есть - S3 вернёт ошибку, её просто игнорируем.
	s.client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})

	return s, nil
}

// Upload кладёт файл в S3 под именем key
func (s *S3Client) Upload(key string, file io.Reader, contentType string) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	return err
}

// Download достаёт файл из S3. После чтения файл нужно закрыть (Close)
func (s *S3Client) Download(key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}
