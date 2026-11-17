package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"Go_project/internal/domain"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// newTestDB подключается к настоящей Mongo и даёт чистую базу под один тест.
// Адрес берём из MONGO_TEST_URI. Если переменной нет, тест пропускается
// (так `go test ./...` не падает на компьютере без Mongo).
func newTestDB(t *testing.T) *mongo.Database {
	t.Helper()

	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI не задан, тест с базой пропущен")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("не подключились к Mongo: %v", err)
	}

	// Уникальное имя базы, чтобы тесты не мешали друг другу и твоим реальным данным
	db := client.Database(fmt.Sprintf("test_%d", time.Now().UnixNano()))

	// После теста базу выбрасываем
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	return db
}

func TestImageList_ReturnsOnlyOwnImagesNewestFirst(t *testing.T) {
	repo := NewMongoImageRepository(newTestDB(t))

	// Две картинки Стаса и одна чужая. Сортировка идёт по _id по убыванию,
	// поэтому "a2" должен оказаться выше "a1".
	for _, img := range []domain.Image{
		{ID: "a1", OwnerUID: "stas", Filename: "first.jpg", Status: "done"},
		{ID: "b1", OwnerUID: "other", Filename: "secret.jpg", Status: "done"},
		{ID: "a2", OwnerUID: "stas", Filename: "second.jpg", Status: "done"},
	} {
		img := img
		if err := repo.Create(&img); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	got, err := repo.List("stas")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ожидали 2 своих картинки, получили %d", len(got))
	}
	for _, img := range got {
		if img.OwnerUID != "stas" {
			t.Errorf("в списке чужая картинка: %+v", img)
		}
	}
	if got[0].ID != "a2" || got[1].ID != "a1" {
		t.Errorf("порядок должен быть новые сверху (a2, a1), получили %s, %s", got[0].ID, got[1].ID)
	}
}

func TestImageList_UnknownOwnerGetsEmptyListNotNil(t *testing.T) {
	repo := NewMongoImageRepository(newTestDB(t))

	got, err := repo.List("nobody")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil {
		t.Fatal("ожидали пустой список [], а получили nil (в JSON это будет null)")
	}
	if len(got) != 0 {
		t.Fatalf("ожидали 0 картинок, получили %d", len(got))
	}
}

func TestImageCount_CountsAllOwners(t *testing.T) {
	repo := NewMongoImageRepository(newTestDB(t))

	for _, img := range []domain.Image{
		{ID: "1", OwnerUID: "stas"},
		{ID: "2", OwnerUID: "other"},
		{ID: "3", OwnerUID: "other"},
	} {
		img := img
		if err := repo.Create(&img); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	n, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Fatalf("ожидали 3, получили %d", n)
	}
}

func TestUserUpsert_CreatesOnceThenOnlyUpdates(t *testing.T) {
	repo := NewMongoUserRepository(newTestDB(t))

	first, err := repo.Upsert("uid-1", "Stas", "stas@example.com")
	if err != nil {
		t.Fatalf("первый Upsert: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // чтобы время второго входа точно отличалось

	second, err := repo.Upsert("uid-1", "Stas New", "new@example.com")
	if err != nil {
		t.Fatalf("второй Upsert: %v", err)
	}

	// Пользователь один и тот же, второго не появилось
	n, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("ожидали 1 пользователя, получили %d", n)
	}

	// Имя и почта обновились
	if second.Name != "Stas New" || second.Email != "new@example.com" {
		t.Errorf("имя/почта не обновились: %+v", second)
	}
	// Дата создания осталась старой, а время входа сдвинулось вперёд
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("created_at изменился: было %v, стало %v", first.CreatedAt, second.CreatedAt)
	}
	if !second.LastLoginAt.After(first.LastLoginAt) {
		t.Errorf("last_login_at не сдвинулся вперёд: %v -> %v", first.LastLoginAt, second.LastLoginAt)
	}
}
