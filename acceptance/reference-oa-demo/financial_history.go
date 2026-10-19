package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

func ensureContractHistory(ctx context.Context, forms *formClient, statePath string, current *state) error {
	path := "/contracts/" + current.ContractID
	if !current.ContractChanged {
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		if strings.Contains(page, "OA-DEMO-SUP-001") {
			current.ContractChanged = true
		} else {
			form, err := formByAction(page, path+"/changes")
			if err != nil {
				return err
			}
			if _, err := forms.post(ctx, path+"/changes", url.Values{
				"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdContractChange},
				"expected_revision": {hiddenValue(form, "expected_revision")}, "effective_order": {hiddenValue(form, "effective_order")},
				"instrument_number": {"OA-DEMO-SUP-001"}, "instrument_title": {"共享会议区设计补充协议"},
				"change_summary": {"增加共享会议区设计范围并调整合同金额"}, "instrument_signed_on": {"2026-10-01"},
				"instrument_effective_on": {"2026-10-02"}, "amount_effect": {"adjustment"}, "amount_value": {"5000.00"},
			}); err != nil {
				return fmt.Errorf("append contract change: %w", err)
			}
			current.ContractChanged = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if !current.ContractCorrected {
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		if strings.Contains(page, "OA-DEMO-COR-001") {
			current.ContractCorrected = true
		} else {
			form, err := formByAction(page, path+"/corrections")
			if err != nil {
				return err
			}
			if _, err := forms.post(ctx, path+"/corrections", url.Values{
				"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdContractCorrection},
				"expected_status": {hiddenValue(form, "expected_status")}, "expected_revision": {hiddenValue(form, "expected_revision")},
				"effective_order": {hiddenValue(form, "effective_order")}, "correction_instrument_number": {"OA-DEMO-COR-001"},
				"correction_instrument_title": {"合同录入更正确认单"}, "correction_reason": {"核对签署原件后补充标题说明，金额与商业条款不变"},
				"correction_instrument_signed_on": {"2026-10-03"}, "correction_instrument_effective_on": {"2026-10-03"},
				"corrected_title": {"青岚办公空间改造设计合同（录入更正）"}, "corrected_category": {"设计服务"},
				"corrected_total_amount": {"125000.00"}, "corrected_signed_on": {"2026-09-25"}, "corrected_effective_on": {"2026-09-25"},
			}); err != nil {
				return fmt.Errorf("correct contract: %w", err)
			}
			current.ContractCorrected = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if !current.ContractTerminated {
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		if strings.Contains(page, "OA-DEMO-TERM-001") && strings.Contains(page, "已终止") {
			current.ContractTerminated = true
		} else {
			form, err := formByAction(page, path+"/terminate")
			if err != nil {
				return err
			}
			if _, err := forms.post(ctx, path+"/terminate", url.Values{
				"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdContractTerminate},
				"expected_revision": {hiddenValue(form, "expected_revision")}, "effective_order": {hiddenValue(form, "effective_order")},
				"termination_instrument_number": {"OA-DEMO-TERM-001"}, "termination_instrument_title": {"合同履行完成确认书"},
				"termination_reason": {"双方确认约定设计范围已完成，进入最终结算"}, "termination_signed_on": {"2026-10-20"},
				"termination_effective_on": {"2026-10-20"},
			}); err != nil {
				return fmt.Errorf("terminate contract: %w", err)
			}
			current.ContractTerminated = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if err := ensureContractSettlement(ctx, forms, statePath, current, path, false); err != nil {
		return err
	}
	if err := ensureContractSettlement(ctx, forms, statePath, current, path, true); err != nil {
		return err
	}
	if current.ContractClosed {
		return nil
	}
	page, err := forms.get(ctx, path)
	if err != nil {
		return err
	}
	if strings.Contains(page, "已关闭") && strings.Contains(page, "最终结算") {
		current.ContractClosed = true
		return writeState(statePath, *current)
	}
	form, err := formByAction(page, path+"/close")
	if err != nil {
		return err
	}
	if _, err := forms.post(ctx, path+"/close", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdContractClose},
		"expected_revision": {hiddenValue(form, "expected_revision")}, "expected_settlement_sequence": {hiddenValue(form, "expected_settlement_sequence")},
		"settlement_disposition": {hiddenValue(form, "settlement_disposition")}, "closed_on": {"2026-10-31"},
		"closure_reason": {"最终交付、结算与资料已经复核，结束常规合同维护"},
	}); err != nil {
		return fmt.Errorf("close contract: %w", err)
	}
	current.ContractClosed = true
	return writeState(statePath, *current)
}

func ensureContractSettlement(ctx context.Context, forms *formClient, statePath string, current *state, path string, final bool) error {
	done := &current.ContractProgressSettled
	command, kind, number, title, amount, confirmedOn, basis := cmdContractProgress, "progress", "OA-DEMO-SETTLE-001", "第一阶段累计结算", "70000.00", "2026-10-25", "双方按已交付成果确认阶段结算"
	if final {
		done = &current.ContractFinalSettled
		command, kind, number, title, amount, confirmedOn, basis = cmdContractFinal, "final", "OA-DEMO-SETTLE-002", "最终累计结算", "125000.00", "2026-10-30", "双方根据完整交付确认最终累计结算金额"
	}
	if *done {
		return nil
	}
	page, err := forms.get(ctx, path)
	if err != nil {
		return err
	}
	if strings.Contains(page, number) {
		*done = true
		return writeState(statePath, *current)
	}
	form, err := formByAction(page, path+"/settlements")
	if err != nil {
		return err
	}
	positions := checkedFormValues(form, "confirming_party_position")
	if len(positions) == 0 {
		return errors.New("contract settlement form did not expose confirming party positions")
	}
	values := url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {command}, "expected_revision": {hiddenValue(form, "expected_revision")},
		"expected_settlement_sequence": {hiddenValue(form, "expected_settlement_sequence")}, "settlement_kind": {kind}, "settlement_number": {number},
		"settlement_title": {title}, "settlement_amount": {amount}, "settlement_confirmed_on": {confirmedOn}, "settlement_basis": {basis},
	}
	for _, position := range positions {
		values.Add("confirming_party_position", position)
	}
	if _, err := forms.post(ctx, path+"/settlements", values); err != nil {
		return fmt.Errorf("record %s contract settlement: %w", kind, err)
	}
	*done = true
	return writeState(statePath, *current)
}

func checkedFormValues(form, name string) []string {
	pattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]+)"`)
	matches := pattern.FindAllStringSubmatch(form, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, html.UnescapeString(match[1]))
	}
	return values
}

func ensureMoneyAdjustment(ctx context.Context, forms *formClient, statePath string, current *state) error {
	if current.MoneyAdjusted {
		return nil
	}
	path := "/obligations/" + current.ObligationID
	page, err := forms.get(ctx, path)
	if err != nil {
		return err
	}
	form, err := formByAction(page, path+"/adjust-amount")
	if err != nil {
		if strings.Contains(page, "Current revision</dt><dd>2</dd>") {
			current.MoneyAdjusted = true
			return writeState(statePath, *current)
		}
		return err
	}
	if _, err := forms.post(ctx, path+"/adjust-amount", url.Values{
		"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdObligationAdjust},
		"expected_current_revision": {hiddenValue(form, "expected_current_revision")}, "adjustment_type": {"increase"},
		"adjustment_amount": {"5000.00"}, "adjustment_reason": {"对应合同补充的共享会议区设计范围"},
	}); err != nil {
		return fmt.Errorf("adjust obligation: %w", err)
	}
	current.MoneyAdjusted = true
	return writeState(statePath, *current)
}

func ensureMoneyHistory(ctx context.Context, forms *formClient, statePath string, current *state) error {
	if !current.AllocationReversed {
		path := "/allocations/" + current.AllocationID
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/reverse")
		if err != nil {
			if strings.Contains(page, "reversed") {
				current.AllocationReversed = true
			} else {
				return err
			}
		} else {
			if _, err := forms.post(ctx, path+"/reverse", url.Values{"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdAllocationReverse}, "reason": {"首次核销依据匹配错误，释放后按复核结果重建"}}); err != nil {
				return fmt.Errorf("reverse allocation: %w", err)
			}
			current.AllocationReversed = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.ReplacementAllocationID == "" {
		page, err := forms.get(ctx, "/allocations")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/allocations", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdReplacementAlloc}, "cash_transaction_id": {current.CashTransactionID},
			"obligation_selection": {current.ObligationID + ":2"}, "amount": {"40000.00"}, "manual_source_reference": {"首期回款复核后重新核销"},
		})
		if err != nil {
			return fmt.Errorf("create replacement allocation: %w", err)
		}
		current.ReplacementAllocationID, err = pathID(location, "/allocations/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.SecondCashTransactionID == "" {
		page, err := forms.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/cash-transactions", oaCashValues(hiddenValue(page, "csrf_token"), cmdSecondCash, current, "青岚项目第二笔阶段收款", "30000.00", "OA-DEMO-BANK-002"))
		if err != nil {
			return fmt.Errorf("create second cash receipt: %w", err)
		}
		current.SecondCashTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.SecondAllocationID == "" {
		page, err := forms.get(ctx, "/allocations")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/allocations", url.Values{
			"csrf_token": {hiddenValue(page, "csrf_token")}, "command_id": {cmdSecondAllocation}, "cash_transaction_id": {current.SecondCashTransactionID},
			"obligation_selection": {current.ObligationID + ":2"}, "amount": {"30000.00"}, "manual_source_reference": {"第二笔阶段回款核销"},
		})
		if err != nil {
			return fmt.Errorf("allocate second receipt: %w", err)
		}
		current.SecondAllocationID, err = pathID(location, "/allocations/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	path := "/obligations/" + current.ObligationID
	if !current.MoneyRemissionRecorded {
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/remissions")
		if err != nil {
			if strings.Contains(page, "55000 CNY") {
				current.MoneyRemissionRecorded = true
			} else {
				return err
			}
		} else {
			if _, err := forms.post(ctx, path+"/remissions", url.Values{
				"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdMoneyRemission},
				"expected_current_revision": {hiddenValue(form, "expected_current_revision")}, "remission_amount": {"55000.00"},
				"remission_reason": {"双方最终确认取消未实施范围"}, "manual_source_reference": {"OA-DEMO-REMISSION-001"},
			}); err != nil {
				return fmt.Errorf("record remission: %w", err)
			}
			current.MoneyRemissionRecorded = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if !current.ObligationClosed {
		page, err := forms.get(ctx, path)
		if err != nil {
			return err
		}
		form, err := formByAction(page, path+"/close")
		if err != nil {
			if strings.Contains(page, "关闭事实") {
				current.ObligationClosed = true
			} else {
				return err
			}
		} else {
			if _, err := forms.post(ctx, path+"/close", url.Values{
				"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdObligationClose},
				"expected_current_revision": {hiddenValue(form, "expected_current_revision")}, "expected_balance_sequence": {hiddenValue(form, "expected_balance_sequence")},
				"closure_reason": {"两笔回款核销和范围减免均已复核，余额归零"},
			}); err != nil {
				return fmt.Errorf("close obligation: %w", err)
			}
			current.ObligationClosed = true
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.ErrorCashTransactionID == "" {
		page, err := forms.get(ctx, "/cash-transactions")
		if err != nil {
			return err
		}
		location, err := forms.post(ctx, "/cash-transactions", oaCashValues(hiddenValue(page, "csrf_token"), cmdErrorCash, current, "误录测试收款", "1000.00", "OA-DEMO-BANK-ERROR-001"))
		if err != nil {
			return fmt.Errorf("create erroneous receipt: %w", err)
		}
		current.ErrorCashTransactionID, err = pathID(location, "/cash-transactions/")
		if err != nil {
			return err
		}
		if err := writeState(statePath, *current); err != nil {
			return err
		}
	}
	if current.ErrorCashReversed {
		return nil
	}
	errorPath := "/cash-transactions/" + current.ErrorCashTransactionID
	page, err := forms.get(ctx, errorPath)
	if err != nil {
		return err
	}
	form, err := formByAction(page, errorPath+"/reverse")
	if err != nil {
		if strings.Contains(page, "资金冲销事实") {
			current.ErrorCashReversed = true
			return writeState(statePath, *current)
		}
		return err
	}
	if _, err := forms.post(ctx, errorPath+"/reverse", url.Values{"csrf_token": {hiddenValue(form, "csrf_token")}, "command_id": {cmdErrorCashReverse}, "reason": {"发现重复录入，保留冲销轨迹"}}); err != nil {
		return fmt.Errorf("reverse erroneous receipt: %w", err)
	}
	current.ErrorCashReversed = true
	return writeState(statePath, *current)
}

func oaCashValues(csrf, command string, current *state, description, amount, externalReference string) url.Values {
	return url.Values{
		"csrf_token": {csrf}, "command_id": {command}, "type": {"receipt"}, "ledger_owner_party_id": {current.MoneyOwnerPartyID},
		"counterparty_party_id": {current.MoneyClientPartyID}, "description": {description}, "amount": {amount}, "currency": {"CNY"},
		"occurred_on": {"2026-09-25"}, "account_reference": {"演示账户尾号 2026"}, "manual_source_reference": {"人工核对银行回单"},
		"external_transaction_reference": {externalReference},
	}
}
