package market

import "github.com/aidenlippert/zerostate/libs/agentcard-go"

func agentDID(card *agentcard.AgentCard) string {
	if card == nil {
		return ""
	}
	return card.CredentialSubject.ID.String()
}
