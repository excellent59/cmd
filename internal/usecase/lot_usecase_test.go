package usecase

import (
	"context"
	"fmt"
	"pro-lot-bot/internal/domain"
	"testing"
)

// MockLotScraper — заглушка для скрапера
type MockLotScraper struct {
	lots []domain.Lot
}

func (m *MockLotScraper) GetLots(ctx context.Context, filter domain.LotFilter) (domain.ScrapeResult, error) {
	// Complete=false, чтобы в тестах не срабатывало удаление «пропавших» лотов.
	return domain.ScrapeResult{Lots: m.lots}, nil
}

// MockLotRepository — заглушка для репозитория
type MockLotRepository struct {
	lots map[string]domain.Lot
}

func NewMockLotRepository() *MockLotRepository {
	return &MockLotRepository{
		lots: make(map[string]domain.Lot),
	}
}

func (m *MockLotRepository) GetCategories() []string {
	return []string{"1", "2"}
}

func (m *MockLotRepository) HasLot(ctx context.Context, lotID string) bool {
	_, exists := m.lots[lotID]
	return exists
}

func (m *MockLotRepository) GetLotPrice(ctx context.Context, lotID string) (string, bool) {
	lot, exists := m.lots[lotID]
	if !exists {
		return "", false
	}
	return lot.Price, true
}

func (m *MockLotRepository) AddLot(ctx context.Context, lot domain.Lot) error {
	m.lots[lot.ID] = lot
	return nil
}

func (m *MockLotRepository) UpdateLotPrice(ctx context.Context, lotID, price string) error {
	lot, exists := m.lots[lotID]
	if !exists {
		return fmt.Errorf("лот не найден")
	}
	lot.Price = price
	m.lots[lotID] = lot
	return nil
}

func (m *MockLotRepository) Close() {
	// Для тестов ничего не делаем
}

func (m *MockLotRepository) MarkLotSent(ctx context.Context, lotID string, chatID int64, messageIDs []int) error {
	if lot, exists := m.lots[lotID]; exists {
		lot.IsSent = true
		m.lots[lotID] = lot
	}
	return nil
}

func (m *MockLotRepository) DeleteRemovedLots(ctx context.Context, seenIDs []string) ([]domain.SentMessageRef, []string, error) {
	return nil, nil, nil
}

// --- Заглушки методов админ-панели (чтобы мок удовлетворял интерфейсу) ---

func (m *MockLotRepository) GetDashboardStats(ctx context.Context) (domain.DashboardStats, error) {
	return domain.DashboardStats{}, nil
}

func (m *MockLotRepository) GetLotsPage(ctx context.Context, q domain.LotQuery) (domain.LotsPage, error) {
	return domain.LotsPage{}, nil
}

func (m *MockLotRepository) GetLotByID(ctx context.Context, id string) (domain.Lot, error) {
	lot, ok := m.lots[id]
	if !ok {
		return domain.Lot{}, domain.ErrLotNotFound
	}
	return lot, nil
}

func (m *MockLotRepository) DeleteLot(ctx context.Context, id string) error {
	delete(m.lots, id)
	return nil
}

func (m *MockLotRepository) GetPriceHistory(ctx context.Context, id string) ([]domain.PriceHistory, error) {
	return nil, nil
}

func (m *MockLotRepository) GetSettings(ctx context.Context) (domain.Settings, error) {
	return domain.Settings{}, domain.ErrNoSettings
}

func (m *MockLotRepository) SaveSettings(ctx context.Context, s domain.Settings) error {
	return nil
}

// ТЕСТЫ

func TestGetNewLots(t *testing.T) {
	mockScraper := &MockLotScraper{
		lots: []domain.Lot{
			{ID: "1", Title: "Test Lot 1", Price: "100000", Year: 2015, URL: "http://test.com/1"},
			{ID: "2", Title: "Test Lot 2", Price: "200000", Year: 2018, URL: "http://test.com/2"},
		},
	}

	mockRepo := NewMockLotRepository()
	usecase := NewLotUsecase(mockScraper, mockRepo)

	lots, err := usecase.GetNewLots(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(lots) != 2 {
		t.Errorf("Expected 2 new lots, got %d", len(lots))
	}

	if !mockRepo.HasLot(context.Background(), "1") {
		t.Error("Lot 1 was not added to repository")
	}
}

func TestGetLotsWithPriceChange(t *testing.T) {
	mockScraper := &MockLotScraper{
		lots: []domain.Lot{
			{ID: "1", Title: "Test Lot 1", Price: "150000", Year: 2015, URL: "http://test.com/1"},
		},
	}

	mockRepo := NewMockLotRepository()

	// Добавляем лот со старой ценой
	oldLot := domain.Lot{
		ID:    "1",
		Price: "100000",
		Year:  2015,
	}
	mockRepo.AddLot(context.Background(), oldLot)

	usecase := NewLotUsecase(mockScraper, mockRepo)

	lots, err := usecase.GetNewLots(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(lots) != 1 {
		t.Errorf("Expected 1 lot (price changed), got %d", len(lots))
	}

	updatedPrice, exists := mockRepo.GetLotPrice(context.Background(), "1")
	if !exists {
		t.Fatal("Lot 1 should exist in repository")
	}
	if updatedPrice != "150000" {
		t.Errorf("Expected price to be updated to 150000, got %s", updatedPrice)
	}
}

func TestGetLotsNoChanges(t *testing.T) {
	mockScraper := &MockLotScraper{
		lots: []domain.Lot{
			{ID: "1", Title: "Test Lot 1", Price: "100000", Year: 2015, URL: "http://test.com/1"},
		},
	}

	mockRepo := NewMockLotRepository()

	oldLot := domain.Lot{
		ID:    "1",
		Price: "100000",
		Year:  2015,
	}
	mockRepo.AddLot(context.Background(), oldLot)

	usecase := NewLotUsecase(mockScraper, mockRepo)

	lots, err := usecase.GetNewLots(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(lots) != 0 {
		t.Errorf("Expected 0 lots (no changes), got %d", len(lots))
	}
}

// GetUnsentLots возвращает все лоты с is_sent = false
func (m *MockLotRepository) GetUnsentLots(ctx context.Context) ([]domain.Lot, error) {
	var unsentLots []domain.Lot
	for _, lot := range m.lots {
		if !lot.IsSent {
			unsentLots = append(unsentLots, lot)
		}
	}
	return unsentLots, nil
}
