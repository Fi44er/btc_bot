package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// serviceError создает кастомную ошибку для нашего сервиса.
type serviceError struct {
	StatusCode int
	Message    string
}

func (e *serviceError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Message)
}

// krakenResponse определяет структуру ответа от API Kraken.
// Мы используем map[string]interface{} для "result", так как ключ ("XXBTZUSD") динамический.
type krakenResponse struct {
	Error  []string                `json:"error"`
	Result map[string]krakenTicker `json:"result"`
}

type krakenTicker struct {
	// c = last trade closed array(<price>, <lot volume>)
	LastTrade []string `json:"c"`
}

// exchangeRateResponse определяет структуру ответа от API обменных курсов.
type exchangeRateResponse struct {
	Result         string             `json:"result"`
	Rates          map[string]float64 `json:"rates"`
	TimeNextUpdate int64              `json:"time_next_update_unix"`
}

// Service управляет получением курсов и кэшированием.
type Service struct {
	httpClient *http.Client

	// Поля для кэширования курса USD/RUB
	mu             sync.Mutex // Защищает доступ к полям кэша
	usdToRubRate   float64
	nextUpdateUnix int64
}

// NewService создает новый экземпляр Service.
func NewService() *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetBTCRUBRate является основной функцией, которая оркестрирует получение курсов.
func (s *Service) GetBTCRUBRate() (float64, error) {
	// Канал для получения результата и ошибки от асинхронных вызовов
	type btcResult struct {
		price float64
		err   error
	}
	btcChan := make(chan btcResult, 1)

	// Шаг 1: Асинхронно получаем курс BTC/USD с Kraken
	go func() {
		btcPrice, err := s.getBTCUSDPrice()
		btcChan <- btcResult{price: btcPrice, err: err}
	}()

	// Шаг 2: Получаем курс USD/RUB (с использованием кэша)
	rubRate, err := s.getUSDRUBRate()
	if err != nil {
		return 0, fmt.Errorf("failed to get USD/RUB rate: %v", err)
	}

	// Ожидаем результат от Kraken
	result := <-btcChan
	if result.err != nil {
		return 0, fmt.Errorf("failed to get BTC/USD price: %v", result.err)
	}
	btcPrice := result.price

	// Шаг 3: Рассчитываем и возвращаем итоговый курс
	finalRate := btcPrice * rubRate
	return finalRate, nil
}

// getBTCUSDPrice отправляет запрос к API Kraken и парсит ответ.
func (s *Service) getBTCUSDPrice() (float64, error) {
	resp, err := s.httpClient.Get("https://api.kraken.com/0/public/Ticker?pair=XBTUSD")
	if err != nil {
		return 0, fmt.Errorf("request to Kraken failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, &serviceError{
			StatusCode: resp.StatusCode,
			Message:    "bad response from Kraken",
		}
	}

	var data krakenResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("failed to parse Kraken response: %v", err)
	}

	// Проверяем на наличие ошибок в ответе API
	if len(data.Error) > 0 {
		return 0, fmt.Errorf("Kraken API error: %v", data.Error)
	}

	// Получаем данные тикера для пары XBTUSD. Kraken использует XXBTZUSD в ответе.
	tickerData, ok := data.Result["XXBTZUSD"]
	if !ok {
		return 0, fmt.Errorf("XXBTZUSD pair not found in Kraken response")
	}

	if len(tickerData.LastTrade) == 0 {
		return 0, fmt.Errorf("price data is missing in Kraken response")
	}

	// Парсим цену из строки
	price, err := strconv.ParseFloat(tickerData.LastTrade[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format from Kraken: %v", err)
	}

	return price, nil
}

// getUSDRUBRate получает курс USD/RUB, используя кэш.
func (s *Service) getUSDRUBRate() (float64, error) {
	// Блокируем мьютекс для безопасной работы с кэшем
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверяем, актуален ли кэш.
	// Если время следующего обновления в будущем, значит кэш свежий.
	if time.Now().Unix() < s.nextUpdateUnix {
		fmt.Println("INFO: Using cached USD/RUB rate.")
		return s.usdToRubRate, nil
	}

	// Если мы здесь, значит кэш устарел или пуст. Делаем новый запрос.
	fmt.Println("INFO: Cache is stale or empty. Fetching new USD/RUB rate...")

	resp, err := s.httpClient.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return 0, fmt.Errorf("request to exchange rate API failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, &serviceError{
			StatusCode: resp.StatusCode,
			Message:    "bad response from exchange rate API",
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read exchange rate response body: %v", err)
	}

	var data exchangeRateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, fmt.Errorf("failed to parse exchange rate response: %v", err)
	}

	if data.Result != "success" {
		return 0, fmt.Errorf("exchange rate API returned an error status: %s", data.Result)
	}

	rubRate, ok := data.Rates["RUB"]
	if !ok {
		return 0, fmt.Errorf("RUB rate not found in exchange rate API response")
	}

	// Обновляем кэш новыми данными
	s.usdToRubRate = rubRate
	s.nextUpdateUnix = data.TimeNextUpdate

	fmt.Printf("INFO: Cache updated. Next update at: %s\n", time.Unix(s.nextUpdateUnix, 0))

	return s.usdToRubRate, nil
}

func main() {
	service := NewService()

	fmt.Println("--- First request ---")
	rate, err := service.GetBTCRUBRate()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Current BTC/RUB rate: %.2f RUB\n", rate)
	}

	fmt.Println("\n--- Second request (should use cache) ---")
	rate, err = service.GetBTCRUBRate()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Current BTC/RUB rate: %.2f RUB\n", rate)
	}
}
