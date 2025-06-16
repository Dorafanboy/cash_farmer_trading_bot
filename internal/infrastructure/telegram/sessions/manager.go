package sessions

import (
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SessionManager struct {
	sessions map[int64]*UserState
	mutex    sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[int64]*UserState),
	}
}

func (sm *SessionManager) GetState(userID int64) *UserState {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if state, exists := sm.sessions[userID]; exists {
		return state
	}

	return &UserState{
		UserID:      userID,
		CurrentFlow: FlowNone,
		Step:        0,
		Data:        make(map[string]interface{}),
		CreatedAt:   time.Now(),
	}
}

func (sm *SessionManager) SetState(userID int64, state *UserState) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.sessions[userID] = state
}

func (sm *SessionManager) UpdateLastMessage(userID int64, message *tgbotapi.Message) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if state, exists := sm.sessions[userID]; exists {
		state.LastMessage = message
	} else {
		sm.sessions[userID] = &UserState{
			UserID:      userID,
			CurrentFlow: FlowNone,
			Step:        0,
			Data:        make(map[string]interface{}),
			LastMessage: message,
			CreatedAt:   time.Now(),
		}
	}
}

func (sm *SessionManager) ClearState(userID int64) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.sessions, userID)
}

func (sm *SessionManager) SetFlow(userID int64, flow FlowType, data map[string]interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if state, exists := sm.sessions[userID]; exists {
		state.CurrentFlow = flow
		state.Step = 0
		if data != nil {
			for k, v := range data {
				state.Data[k] = v
			}
		}
	} else {
		if data == nil {
			data = make(map[string]interface{})
		}
		sm.sessions[userID] = &UserState{
			UserID:      userID,
			CurrentFlow: flow,
			Step:        0,
			Data:        data,
			CreatedAt:   time.Now(),
		}
	}
}

func (sm *SessionManager) NextStep(userID int64) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if state, exists := sm.sessions[userID]; exists {
		state.Step++
	}
}

func (sm *SessionManager) SetData(userID int64, key string, value interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if state, exists := sm.sessions[userID]; exists {
		state.Data[key] = value
	} else {
		// Создаем новое состояние если его нет
		sm.sessions[userID] = &UserState{
			UserID:      userID,
			CurrentFlow: FlowNone,
			Step:        0,
			Data:        map[string]interface{}{key: value},
			CreatedAt:   time.Now(),
		}
	}
}

func (sm *SessionManager) GetData(userID int64, key string) interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if state, exists := sm.sessions[userID]; exists {
		return state.Data[key]
	}
	return nil
}

func (sm *SessionManager) GetLastMessage(userID int64) *tgbotapi.Message {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if state, exists := sm.sessions[userID]; exists {
		return state.LastMessage
	}
	return nil
}
