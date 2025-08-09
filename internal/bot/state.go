package bot

const (
	stateDefault            = ""
	stateAwaitingCardNumber = "awaiting_card_number"
)

var (
	testnetAPIURL = "https://mempool.space/testnet4/api"
	mainnetAPIURL = "https://mempool.space/api"
)

func (b *Bot) setState(userID int64, state string) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	b.userStates[userID] = state
	b.logger.Debugf("Set state for user %d: %s", userID, state)
}

func (b *Bot) getUserState(userID int64) string {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	return b.userStates[userID]
}
