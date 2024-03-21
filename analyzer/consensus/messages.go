package consensus

import (
	"encoding/json"

	"github.com/oasisprotocol/nexus/analyzer/util/addresses"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	"github.com/oasisprotocol/nexus/log"
)

type MessageData struct {
	messageType      apiTypes.RoothashMessageType
	body             json.RawMessage
	addressPreimages map[apiTypes.Address]*addresses.PreimageData
	relatedAddresses map[apiTypes.Address]struct{}
}

func extractMessageData(logger *log.Logger, m message.Message) MessageData {
	messageData := MessageData{
		addressPreimages: map[apiTypes.Address]*addresses.PreimageData{},
		relatedAddresses: map[apiTypes.Address]struct{}{},
	}
	switch {
	case m.Staking != nil:
		switch {
		case m.Staking.Transfer != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeStakingTransfer
			body, err := json.Marshal(m.Staking.Transfer)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
			_, err = addresses.RegisterRelatedOCSAddress(messageData.relatedAddresses, m.Staking.Transfer.To)
			if err != nil {
				logger.Info("register related address 'to' failed",
					"message_type", messageData.messageType,
					"err", err,
				)
			}
		case m.Staking.Withdraw != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeStakingWithdraw
			body, err := json.Marshal(m.Staking.Withdraw)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
			_, err = addresses.RegisterRelatedOCSAddress(messageData.relatedAddresses, m.Staking.Withdraw.From)
			if err != nil {
				logger.Info("register related address 'from' failed",
					"message_type", messageData.messageType,
					"err", err,
				)
			}
		case m.Staking.AddEscrow != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeStakingAddEscrow
			body, err := json.Marshal(m.Staking.AddEscrow)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
			_, err = addresses.RegisterRelatedOCSAddress(messageData.relatedAddresses, m.Staking.AddEscrow.Account)
			if err != nil {
				logger.Info("register related address 'account' failed",
					"message_type", messageData.messageType,
					"err", err,
				)
			}
		case m.Staking.ReclaimEscrow != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeStakingReclaimEscrow
			body, err := json.Marshal(m.Staking.ReclaimEscrow)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
			_, err = addresses.RegisterRelatedOCSAddress(messageData.relatedAddresses, m.Staking.ReclaimEscrow.Account)
			if err != nil {
				logger.Info("register related address 'account' failed",
					"message_type", messageData.messageType,
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
			messageData.messageType = apiTypes.RoothashMessageTypeRegistryUpdateRuntime
			body, err := json.Marshal(m.Registry.UpdateRuntime)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
		}
	case m.Governance != nil:
		switch {
		case m.Governance.CastVote != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeGovernanceCastVote
			body, err := json.Marshal(m.Governance.CastVote)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
		case m.Governance.SubmitProposal != nil:
			messageData.messageType = apiTypes.RoothashMessageTypeGovernanceSubmitProposal
			body, err := json.Marshal(m.Governance.SubmitProposal)
			if err != nil {
				logger.Info("marshal message body failed",
					"message_type", messageData.messageType,
					"err", err,
				)
				break
			}
			messageData.body = body
		}
	default:
		logger.Info("unhandled message",
			"message", m,
		)
	}
	return messageData
}
