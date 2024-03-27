package consensus

import (
	"encoding/json"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	"github.com/oasisprotocol/nexus/log"
)

type parsedMessage struct {
	messageType      apiTypes.RoothashMessageType
	body             json.RawMessage
	relatedAddresses map[apiTypes.Address]struct{}
}

func extractMessageData(logger *log.Logger, messageRaw json.RawMessage) parsedMessage {
	pm := parsedMessage{
		relatedAddresses: map[apiTypes.Address]struct{}{},
	}
	var m message.Message
	if err := json.Unmarshal(messageRaw, &m); err != nil {
		logger.Info("unmarshal message failed",
			"err", err,
		)
		return pm
	}
	switch {
	case m.Staking != nil:
		switch {
		case m.Staking.Transfer != nil:
			pm.messageType = apiTypes.RoothashMessageTypeStakingTransfer
			body, err := json.Marshal(m.Staking.Transfer)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
			_, err = registerRelatedOCAddress(pm.relatedAddresses, m.Staking.Transfer.To)
			if err != nil {
				logger.Info("register related address 'to' failed",
					"message_type", pm.messageType,
					"err", err,
				)
			}
		case m.Staking.Withdraw != nil:
			pm.messageType = apiTypes.RoothashMessageTypeStakingWithdraw
			body, err := json.Marshal(m.Staking.Withdraw)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
			_, err = registerRelatedOCAddress(pm.relatedAddresses, m.Staking.Withdraw.From)
			if err != nil {
				logger.Info("register related address 'from' failed",
					"message_type", pm.messageType,
					"err", err,
				)
			}
		case m.Staking.AddEscrow != nil:
			pm.messageType = apiTypes.RoothashMessageTypeStakingAddEscrow
			body, err := json.Marshal(m.Staking.AddEscrow)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
			_, err = registerRelatedOCAddress(pm.relatedAddresses, m.Staking.AddEscrow.Account)
			if err != nil {
				logger.Info("register related address 'account' failed",
					"message_type", pm.messageType,
					"err", err,
				)
			}
		case m.Staking.ReclaimEscrow != nil:
			pm.messageType = apiTypes.RoothashMessageTypeStakingReclaimEscrow
			body, err := json.Marshal(m.Staking.ReclaimEscrow)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
			_, err = registerRelatedOCAddress(pm.relatedAddresses, m.Staking.ReclaimEscrow.Account)
			if err != nil {
				logger.Info("register related address 'account' failed",
					"message_type", pm.messageType,
					"err", err,
				)
			}
		default:
			logger.Info("unhandled staking message",
				"staking_message", m.Staking,
			)
		}
	case m.Registry != nil:
		switch {
		case m.Registry.UpdateRuntime != nil:
			pm.messageType = apiTypes.RoothashMessageTypeRegistryUpdateRuntime
			body, err := json.Marshal(m.Registry.UpdateRuntime)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
		}
	case m.Governance != nil:
		switch {
		case m.Governance.CastVote != nil:
			pm.messageType = apiTypes.RoothashMessageTypeGovernanceCastVote
			body, err := json.Marshal(m.Governance.CastVote)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
		case m.Governance.SubmitProposal != nil:
			pm.messageType = apiTypes.RoothashMessageTypeGovernanceSubmitProposal
			body, err := json.Marshal(m.Governance.SubmitProposal)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", pm.messageType,
					"err", err,
				)
				break
			}
			pm.body = body
		}
	default:
		logger.Info("unhandled message",
			"message", m,
		)
	}
	return pm
}
