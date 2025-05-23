package ledger

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/segmentio/ksuid"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	NilUsers          = "NilUsers"
	LedgerTable       = "LedgerTable"
	TransactionsTable = "TransactionsTable"
)

// Balances represents the amount of money in a user's account.
// AccountID is a unique identifier for the account, and Amount
// is the balance available in the account.
type Balances struct {
	AccountID string  `json:"AccountID"`
	Amount    float64 `json:"Amount"`
	// add meta-fields here
}

// UserBalance represents the user's balance in the DynamoDB table.
// It includes the AccountID and the associated Amount.
type UserBalance struct {
	AccountID string  `json:"AccountID"`
	Amount    float64 `json:"Amount"`
}

// CheckUsersExist checks if the provided account IDs exist in the DynamoDB table.
// It takes a DynamoDB client and a slice of account IDs and returns a slice of
// non-existent account IDs and an error, if any.
func CheckUsersExist(context context.Context, dbSvc *dynamodb.Client, tenantId string, accountIds []string) ([]string, error) {
	// Prepare the input for the BatchGetItem operation
	if tenantId == "" {
		tenantId = "nil"
	}
	keys := make([]map[string]types.AttributeValue, len(accountIds))
	var err error
	for i, accountId := range accountIds {
		keys[i] = map[string]types.AttributeValue{
			"AccountID": &types.AttributeValueMemberS{Value: accountId},
			"TenantID":  &types.AttributeValueMemberS{Value: tenantId},
		}
	}
	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			NilUsers: {
				Keys: keys,
			},
		},
	}

	// Retrieve the items from DynamoDB
	result, err := dbSvc.BatchGetItem(context, input)
	if err != nil {
		return nil, err
	}

	var notFoundUsers []string
	var foundIds []string
	for _, item := range result.Responses[NilUsers] {
		if item != nil {
			foundIds = append(foundIds, item["AccountID"].(*types.AttributeValueMemberS).Value)
		}
	}

	for _, val := range accountIds {
		if !slices.Contains(foundIds, val) {
			notFoundUsers = append(notFoundUsers, val)
			err = errors.New("user_not_found")
		}
	}

	return notFoundUsers, err
}

// CreateAccountWithBalance creates a new user account with an initial balance.
// It takes a DynamoDB client, an account ID, and an amount to be set as the initial
// balance. It returns an error if the account creation fails.
//
// FIXME(adonese): currently this creates a destructive operation where it overrides an existing user.
// the only way we're yet allowing this, is because the logic is managed via another indirection layer.
func CreateAccountWithBalance(context context.Context, dbSvc *dynamodb.Client, tenantId, accountId string, amount float64) error {
	if tenantId == "" {
		tenantId = "nil" // default value for old clients
	}
	log.Printf("the tenant id is: %s", tenantId)
	item := map[string]types.AttributeValue{
		"AccountID":           &types.AttributeValueMemberS{Value: accountId},
		"full_name":           &types.AttributeValueMemberS{Value: "test-account"},
		"birthday":            &types.AttributeValueMemberS{Value: ""},
		"city":                &types.AttributeValueMemberS{Value: ""},
		"dependants":          &types.AttributeValueMemberN{Value: "0"},
		"income_last_year":    &types.AttributeValueMemberN{Value: "0"},
		"enroll_smes_program": &types.AttributeValueMemberBOOL{Value: false},
		"confirm":             &types.AttributeValueMemberBOOL{Value: false},
		"external_auth":       &types.AttributeValueMemberBOOL{Value: false},
		"password":            &types.AttributeValueMemberS{Value: ""},
		"created_at":          &types.AttributeValueMemberS{Value: time.Now().Local().String()},
		"is_verified":         &types.AttributeValueMemberBOOL{Value: true},
		"id_type":             &types.AttributeValueMemberS{Value: ""},
		"mobile_number":       &types.AttributeValueMemberS{Value: ""},
		"id_number":           &types.AttributeValueMemberS{Value: ""},
		"pic_id_card":         &types.AttributeValueMemberS{Value: ""},
		"amount":              &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", amount)},
		"currency":            &types.AttributeValueMemberS{Value: "SDG"},
		"Version":             &types.AttributeValueMemberN{Value: strconv.FormatInt(getCurrentTimestamp(), 10)},
		"TenantID":            &types.AttributeValueMemberS{Value: tenantId},
	}

	conditionExpression := "attribute_not_exists(AccountID) AND attribute_not_exists(TenantID)"

	// Put the item into the DynamoDB table
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(NilUsers),
		Item:                item,
		ConditionExpression: &conditionExpression,
	}

	_, err := dbSvc.PutItem(context, input)
	log.Printf("the error is: %v", err)
	return err
}

func CreateAccount(context context.Context, dbSvc *dynamodb.Client, tenantId string, user User) error {
	if tenantId == "" {
		tenantId = "nil"
	}
	item := map[string]types.AttributeValue{
		"AccountID":           &types.AttributeValueMemberS{Value: user.AccountID},
		"full_name":           &types.AttributeValueMemberS{Value: user.FullName},
		"birthday":            &types.AttributeValueMemberS{Value: user.Birthday},
		"city":                &types.AttributeValueMemberS{Value: user.City},
		"dependants":          &types.AttributeValueMemberN{Value: strconv.Itoa(user.Dependants)},
		"income_last_year":    &types.AttributeValueMemberN{Value: strconv.Itoa(int(user.IncomeLastYear))},
		"enroll_smes_program": &types.AttributeValueMemberBOOL{Value: user.EnrollSMEsProgram},
		"confirm":             &types.AttributeValueMemberBOOL{Value: user.Confirm},
		"external_auth":       &types.AttributeValueMemberBOOL{Value: user.ExternalAuth},
		"password":            &types.AttributeValueMemberS{Value: user.Password},
		"created_at":          &types.AttributeValueMemberS{Value: time.Now().Local().String()},
		"is_verified":         &types.AttributeValueMemberBOOL{Value: user.IsVerified},
		"id_type":             &types.AttributeValueMemberS{Value: user.IDType},
		"mobile_number":       &types.AttributeValueMemberS{Value: user.MobileNumber},
		"id_number":           &types.AttributeValueMemberS{Value: user.IDNumber},
		"pic_id_card":         &types.AttributeValueMemberS{Value: user.PicIDCard},
		"amount":              &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", user.Amount)},
		"currency":            &types.AttributeValueMemberS{Value: "SDG"},
		"Version":             &types.AttributeValueMemberN{Value: strconv.FormatInt(getCurrentTimestamp(), 10)},
		"TenantID":            &types.AttributeValueMemberS{Value: tenantId},
	}

	// Put the item into the DynamoDB table
	input := &dynamodb.PutItemInput{
		TableName: aws.String(NilUsers),
		Item:      item,
	}

	_, err := dbSvc.PutItem(context, input)
	log.Printf("the error is: %v", err)
	return err
}

// GetAccount retrieves an account by tenant ID and account ID.
func GetAccount(ctx context.Context, dbSvc *dynamodb.Client, trEntry TransactionEntry) (*User, error) {
	if trEntry.TenantID == "" {
		trEntry.TenantID = "nil"
	}
	key := map[string]types.AttributeValue{
		"TenantID":  &types.AttributeValueMemberS{Value: trEntry.TenantID},
		"AccountID": &types.AttributeValueMemberS{Value: trEntry.AccountID},
	}

	result, err := dbSvc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("NilUsers"),
		Key:       key,
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, errors.New("uncaught error: empty user!")
	}

	var user User
	err = attributevalue.UnmarshalMap(result.Item, &user)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %v", err)
	}

	return &user, nil
}

// InquireBalance inquires the balance of a given user account.
// It takes a DynamoDB client and an account ID, returning the balance
// as a float64 and an error if the inquiry fails or the user does not exist.
func InquireBalance(context context.Context, dbSvc *dynamodb.Client, tenantId, AccountID string) (float64, error) {
	if tenantId == "" {
		tenantId = "nil"
	}
	result, err := dbSvc.GetItem(context, &dynamodb.GetItemInput{
		TableName: aws.String(NilUsers),
		Key: map[string]types.AttributeValue{
			"AccountID": &types.AttributeValueMemberS{Value: AccountID},
			"TenantID":  &types.AttributeValueMemberS{Value: tenantId},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to inquire balance for user %s: %v", AccountID, err)
	}
	if result.Item == nil {
		return 0, fmt.Errorf("user %s does not exist", AccountID)
	}
	userBalance := UserBalance{}
	err = attributevalue.UnmarshalMap(result.Item, &userBalance)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal user balance for user %s: %v", AccountID, err)
	}
	return userBalance.Amount, nil
}

// TransferCredits transfers a specified amount from one account to another.
// It performs a transaction that debits one account and credits another.
// It takes a DynamoDB client, the account IDs for the sender and receiver, and
// the amount to transfer. It returns a NilResponse and an error if the transfer fails due to
// insufficient funds or other issues.
func TransferCredits(context context.Context, dbSvc *dynamodb.Client, trEntry TransactionEntry) (NilResponse, error) {
	var response NilResponse
	if trEntry.AccountID == "" {
		return response, errors.New("you must provide Account ID, substitute it for FromAccount to mimic the older api")
	}
	if trEntry.TenantID == "" {
		trEntry.TenantID = "nil"
	}
	timestamp := getCurrentTimestamp()
	var transactionStatus int = 1
	uid := ksuid.New().String()

	transaction := TransactionEntry{
		TenantID:            trEntry.TenantID,
		AccountID:           trEntry.FromAccount,
		SystemTransactionID: uid,
		FromAccount:         trEntry.FromAccount,
		ToAccount:           trEntry.ToAccount,
		Amount:              trEntry.Amount,
		Comment:             "Transfer credits",
		TransactionDate:     timestamp,
		Status:              &transactionStatus,
		InitiatorUUID:       trEntry.InitiatorUUID,
	}

	// Fetch sender account
	sender, err := GetAccount(context, dbSvc, trEntry)
	if err != nil || sender == nil {
		SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus)
		response = NilResponse{
			Status:    "error",
			Code:      "user_not_found",
			Message:   "Error in retrieving sender.",
			Details:   fmt.Sprintf("Error in retrieving sender: %v", err),
			Timestamp: trEntry.Timestamp,
			Data: data{
				UUID:       trEntry.InitiatorUUID,
				SignedUUID: trEntry.SignedUUID,
			},
		}
		return response, err
	}

	// Fetch receiver account
	trEntry.AccountID = trEntry.ToAccount
	receiver, err := GetAccount(context, dbSvc, trEntry)
	if err != nil || receiver == nil {
		SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus)
		response = NilResponse{
			Status:    "error",
			Code:      "user_not_found",
			Message:   "Error in retrieving receiver.",
			Details:   fmt.Sprintf("Error in retrieving receiver: %v", err),
			Timestamp: trEntry.Timestamp,
			Data: data{
				UUID:       trEntry.InitiatorUUID,
				SignedUUID: trEntry.SignedUUID,
			},
		}
		return response, err
	}

	if trEntry.Amount > sender.Amount {
		SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus)
		response = NilResponse{
			Status:    "error",
			Code:      "insufficient_balance",
			Message:   "Insufficient balance to complete the transaction.",
			Details:   "The sender does not have enough balance in their account.",
			Timestamp: trEntry.Timestamp,
			Data: data{
				UUID:       trEntry.InitiatorUUID,
				SignedUUID: trEntry.SignedUUID,
			},
		}
		return response, errors.New("insufficient balance")
	}

	debitEntry := LedgerEntry{
		TenantID:            trEntry.TenantID,
		AccountID:           trEntry.FromAccount,
		Amount:              trEntry.Amount,
		SystemTransactionID: uid,
		Type:                "debit",
		Time:                timestamp,
		InitiatorUUID:       trEntry.InitiatorUUID,
	}
	creditEntry := LedgerEntry{
		TenantID:            trEntry.TenantID,
		AccountID:           trEntry.ToAccount,
		Amount:              trEntry.Amount,
		SystemTransactionID: uid,
		Type:                "credit",
		Time:                timestamp,
		InitiatorUUID:       trEntry.InitiatorUUID,
	}

	avDebit, err := attributevalue.MarshalMap(debitEntry)
	if err != nil {
		return response, fmt.Errorf("failed to marshal ledger entry: %v", err)
	}
	avCredit, err := attributevalue.MarshalMap(creditEntry)
	if err != nil {
		return response, fmt.Errorf("failed to marshal ledger entry: %v", err)
	}

	debitInput := &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Update: &types.Update{
					TableName: aws.String(NilUsers),
					Key: map[string]types.AttributeValue{
						"TenantID":  &types.AttributeValueMemberS{Value: trEntry.TenantID},
						"AccountID": &types.AttributeValueMemberS{Value: trEntry.FromAccount},
					},
					UpdateExpression:    aws.String("SET amount = amount - :amount, Version = :newVersion"),
					ConditionExpression: aws.String("attribute_not_exists(Version) OR Version = :oldVersion"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":amount":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", trEntry.Amount)},
						":oldVersion": &types.AttributeValueMemberN{Value: strconv.FormatInt(sender.Version, 10)},
						":newVersion": &types.AttributeValueMemberN{Value: strconv.FormatInt(getCurrentTimestamp(), 10)},
					},
				},
			},
			{Put: &types.Put{
				TableName: aws.String(LedgerTable),
				Item:      avDebit,
			}},
		},
	}

	_, err = dbSvc.TransactWriteItems(context, debitInput)
	if err != nil {
		transactionStatus = 1
		if err := SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus); err != nil {
			panic(err)
		}
		response = NilResponse{
			Status:    "error",
			Code:      "debit_failed",
			Message:   fmt.Sprintf("Failed to debit from balance for user %s", trEntry.FromAccount),
			Details:   fmt.Sprintf("Error: %v", err),
			Timestamp: trEntry.Timestamp,
			Data: data{
				UUID:       trEntry.InitiatorUUID,
				SignedUUID: trEntry.SignedUUID,
			},
		}
		return response, fmt.Errorf("failed to debit from balance for user %s: %v", trEntry.FromAccount, err)
	}

	creditInput := &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Update: &types.Update{
					TableName: aws.String(NilUsers),
					Key: map[string]types.AttributeValue{
						"TenantID":  &types.AttributeValueMemberS{Value: trEntry.TenantID},
						"AccountID": &types.AttributeValueMemberS{Value: trEntry.ToAccount},
					},
					UpdateExpression:    aws.String("SET amount = amount + :amount, Version = :newVersion"),
					ConditionExpression: aws.String("attribute_exists(AccountID) AND TenantID = :tenantID"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":amount":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", trEntry.Amount)},
						":newVersion": &types.AttributeValueMemberN{Value: strconv.FormatInt(getCurrentTimestamp(), 10)},
						":tenantID":   &types.AttributeValueMemberS{Value: trEntry.TenantID},
					},
				},
			},
			{Put: &types.Put{
				TableName: aws.String(LedgerTable),
				Item:      avCredit,
			}},
		},
	}

	_, err = dbSvc.TransactWriteItems(context, creditInput)
	if err != nil {
		rollbackInput := &dynamodb.UpdateItemInput{
			TableName: aws.String(NilUsers),
			Key: map[string]types.AttributeValue{
				"TenantID":  &types.AttributeValueMemberS{Value: trEntry.TenantID},
				"AccountID": &types.AttributeValueMemberS{Value: trEntry.FromAccount},
			},
			UpdateExpression:    aws.String("SET amount = amount + :amount, Version = :newVersion"),
			ConditionExpression: aws.String("attribute_not_exists(Version) OR Version = :oldVersion"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":amount":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", trEntry.Amount)},
				":oldVersion": &types.AttributeValueMemberN{Value: strconv.FormatInt(sender.Version, 10)},
				":newVersion": &types.AttributeValueMemberN{Value: strconv.FormatInt(getCurrentTimestamp(), 10)},
			},
		}

		_, rollbackErr := dbSvc.UpdateItem(context, rollbackInput)
		if rollbackErr != nil {
			panic(fmt.Errorf("failed to rollback debit for user %s: %v", trEntry.FromAccount, rollbackErr))
		}

		transactionStatus = 1
		if err := SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus); err != nil {
			panic(err)
		}
		response = NilResponse{
			Status:    "error",
			Code:      "credit_failed",
			Message:   fmt.Sprintf("Failed to credit to balance for user %s", trEntry.ToAccount),
			Details:   fmt.Sprintf("Error: %v", err),
			Timestamp: trEntry.Timestamp,
			Data: data{
				UUID:       trEntry.InitiatorUUID,
				SignedUUID: trEntry.SignedUUID,
			},
		}
		return response, fmt.Errorf("failed to credit to balance for user %s: %v", trEntry.ToAccount, err)
	}

	transactionStatus = 0
	if err := SaveToTransactionTable(dbSvc, trEntry.TenantID, transaction, transactionStatus); err != nil {
		panic(err)
	}

	response = NilResponse{
		Status:  "success",
		Code:    "successful_transaction",
		Message: "Transaction initiated successfully.",
		Data: data{
			TransactionID: uid,
			Amount:        trEntry.Amount,
			Currency:      "SDG",
			UUID:          trEntry.InitiatorUUID,
			SignedUUID:    trEntry.SignedUUID,
		},
	}

	return response, nil
}

// GetTransactions retrieves a list of transactions for a specified tenant and account.
// It takes a DynamoDB client, a tenant ID, an account ID, a limit for the number of transactions
// to retrieve, and an optional lastTransactionID for pagination.
// It returns a slice of LedgerEntry, the ID of the last transaction, and an error, if any.
func GetTransactions(context context.Context, dbSvc *dynamodb.Client, tenantID, accountID string, limit int32, lastTransactionID string) ([]LedgerEntry, string, error) {
	if tenantID == "" {
		tenantID = "nil"
	}
	input := &dynamodb.QueryInput{
		TableName:              aws.String("TransactionsTable"),
		KeyConditionExpression: aws.String("TenantID = :tenantId AND AccountID = :accountId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tenantId":  &types.AttributeValueMemberS{Value: tenantID},
			":accountId": &types.AttributeValueMemberS{Value: accountID},
		},
		Limit: aws.Int32(limit),
	}

	// If a lastTransactionID was provided, include it in the input
	if lastTransactionID != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"TenantID":      &types.AttributeValueMemberS{Value: tenantID},
			"TransactionID": &types.AttributeValueMemberS{Value: lastTransactionID},
		}
	}

	// Execute the query
	resp, err := dbSvc.Query(context, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch transactions: %v", err)
	}

	// Unmarshal the items
	var transactions []LedgerEntry
	err = attributevalue.UnmarshalListOfMaps(resp.Items, &transactions)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal transactions: %v", err)
	}

	// If there are more items to be fetched, return the TransactionID of the last item
	var newLastTransactionID string
	if resp.LastEvaluatedKey != nil {
		newLastTransactionID = resp.LastEvaluatedKey["TransactionID"].(*types.AttributeValueMemberS).Value
	}

	return transactions, newLastTransactionID, nil
}

// GetDetailedTransactions retrieves a list of transactions for a specified tenant and account.
// It takes a DynamoDB client, a tenant ID, an account ID, and a limit for the number of transactions
// to retrieve. It returns a slice of TransactionEntry and an error, if any.
func GetDetailedTransactions(context context.Context, dbSvc *dynamodb.Client, tenantID, accountID string, limit int32) ([]TransactionEntry, error) {
	// Query for transactions sent by the account
	if tenantID == "" {
		tenantID = "nil"
	}
	sentTransactions, _, err := getTransactionsByIndex(context, dbSvc, tenantID, "FromAccountIndex", "FromAccount", accountID, limit, "")
	if err != nil {
		return nil, err
	}
	// Query for transactions received by the account
	receivedTransactions, _, err := getTransactionsByIndex(context, dbSvc, tenantID, "ToAccountIndex", "ToAccount", accountID, limit, "")
	if err != nil {
		return nil, err
	}

	// Combine the transactions into a single list
	allTransactions := append(sentTransactions, receivedTransactions...)

	return allTransactions, nil
}

// getTransactionsByIndex is a helper function that queries for transactions on a specific index.
func getTransactionsByIndex(context context.Context, dbSvc *dynamodb.Client, tenantID, indexName, attributeName, accountID string, limit int32, lastTransactionID string) ([]TransactionEntry, string, error) {
	if tenantID == "" {
		tenantID = "nil"
	}
	input := &dynamodb.QueryInput{
		TableName:              aws.String("TransactionsTable"),
		IndexName:              aws.String(indexName),
		KeyConditionExpression: aws.String("TenantID = :tenantId AND " + attributeName + " = :accountId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tenantId":  &types.AttributeValueMemberS{Value: tenantID},
			":accountId": &types.AttributeValueMemberS{Value: accountID},
		},
		Limit:            aws.Int32(limit),
		ScanIndexForward: aws.Bool(false),
	}

	if lastTransactionID != "" {
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"TenantID":      &types.AttributeValueMemberS{Value: tenantID},
			"TransactionID": &types.AttributeValueMemberS{Value: lastTransactionID},
		}
	}

	resp, err := dbSvc.Query(context, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch transactions: %v", err)
	}

	var transactions []TransactionEntry
	err = attributevalue.UnmarshalListOfMaps(resp.Items, &transactions)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal transactions: %v", err)
	}

	var newLastTransactionID string
	if resp.LastEvaluatedKey != nil {
		newLastTransactionID = resp.LastEvaluatedKey["TransactionID"].(*types.AttributeValueMemberS).Value
	}

	return transactions, newLastTransactionID, nil
}

// GetTransaction retrieves a single transaction by its composite key
func GetTransaction(ctx context.Context, dbSvc *dynamodb.Client, tenantID, accountID, systemTransactionID string) (*TransactionEntry, error) {
    // Try GetItem first (optimal if SystemTransactionID is the sort key)
    getInput := &dynamodb.GetItemInput{
        TableName: aws.String("TransactionsTable"),
        Key: map[string]types.AttributeValue{
            "TenantID":    &types.AttributeValueMemberS{Value: tenantID},
            "TransactionID": &types.AttributeValueMemberS{Value: systemTransactionID},
        },
    }
    result, err := dbSvc.GetItem(ctx, getInput)
    if err != nil {
        return nil, fmt.Errorf("GetItem failed: %w", err)
    }
    if result.Item != nil {
        return unmarshalTransaction(result.Item)
    }

    // Fall back to Query if GetItem didn't find it
    queryInput := &dynamodb.QueryInput{
        TableName:              aws.String("TransactionsTable"),
        KeyConditionExpression: aws.String("TenantID = :tenantId AND AccountID = :accountId"),
        FilterExpression:       aws.String("TransactionID = :systemTxId"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":tenantId":   &types.AttributeValueMemberS{Value: tenantID},
            ":accountId":  &types.AttributeValueMemberS{Value: accountID},
            ":systemTxId": &types.AttributeValueMemberS{Value: systemTransactionID},
        },
        Limit: aws.Int32(1),
    }
    queryResult, err := dbSvc.Query(ctx, queryInput)
    if err != nil {
        return nil, fmt.Errorf("Query failed: %w", err)
    }
    if len(queryResult.Items) == 0 {
        return nil, nil // Not found
    }
    return unmarshalTransaction(queryResult.Items[0])
}

// UpdateTransaction updates specific fields of a transaction
func UpdateTransaction(
    ctx context.Context,
    dbSvc *dynamodb.Client,
    tenantID string,
    systemTransactionID string,
    updates map[string]interface{},
) (*TransactionEntry, error) {

    if tenantID == "" {
        tenantID = "nil"
    }

    // 1. Prepare update expression
    updateExpr := "SET "
    attrValues := make(map[string]types.AttributeValue)
    attrNames := make(map[string]string)
    
    i := 0
    for field, value := range updates {
        placeholder := fmt.Sprintf(":val%d", i)
        namePlaceholder := fmt.Sprintf("#field%d", i)
        
        updateExpr += fmt.Sprintf("%s = %s, ", namePlaceholder, placeholder)
        attrValues[placeholder] = createAttributeValue(value)
        attrNames[namePlaceholder] = field
        
        i++
    }
    updateExpr = strings.TrimSuffix(updateExpr, ", ")

    // 2. Execute update
    input := &dynamodb.UpdateItemInput{
        TableName: aws.String("TransactionsTable"),
        Key: map[string]types.AttributeValue{
            "TenantID":      &types.AttributeValueMemberS{Value: tenantID},
            "TransactionID": &types.AttributeValueMemberS{Value: systemTransactionID},
        },
        UpdateExpression:          aws.String(updateExpr),
        ExpressionAttributeValues: attrValues,
        ExpressionAttributeNames:  attrNames,
        ReturnValues:              types.ReturnValueAllNew,
    }

    result, err := dbSvc.UpdateItem(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("failed to update transaction: %w", err)
    }

    // 3. Unmarshal and return updated transaction
    var updatedTx TransactionEntry
    err = attributevalue.UnmarshalMap(result.Attributes, &updatedTx)
    if err != nil {
        return nil, fmt.Errorf("failed to unmarshal updated transaction: %w", err)
    }

    return &updatedTx, nil
}

// Helper function to create AttributeValue from interface{}
func createAttributeValue(value interface{}) types.AttributeValue {
    switch v := value.(type) {
    case string:
        return &types.AttributeValueMemberS{Value: v}
    case float64:
        return &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", v)}
    case int:
        return &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", v)}
    case bool:
        return &types.AttributeValueMemberBOOL{Value: v}
    case time.Time:
        return &types.AttributeValueMemberS{Value: v.Format(time.RFC3339)}
    default:
        return &types.AttributeValueMemberNULL{Value: true}
    }
}

func unmarshalTransaction(item map[string]types.AttributeValue) (*TransactionEntry, error) {
    var tx TransactionEntry
    if err := attributevalue.UnmarshalMap(item, &tx); err != nil {
        return nil, fmt.Errorf("unmarshal failed: %w", err)
    }
    return &tx, nil
}

func GetAllNilTransactions(ctx context.Context, dbSvc *dynamodb.Client, tenantId string, filter TransactionFilter) ([]TransactionEntry, map[string]types.AttributeValue, error) {
	if tenantId == "" {
		tenantId = "nil"
	}

	expressionAttributeValues := map[string]types.AttributeValue{
		":tenantId": &types.AttributeValueMemberS{Value: tenantId},
	}
	expressionAttributeNames := map[string]string{
		"#tenantID": "TenantID",
	}

	keyConditionExpression := "#tenantID = :tenantId"
	filterExpressions := []string{}

	// Determine which index to use based on the filter
	var indexName *string
	if filter.AccountID != "" {
		// Since we can't determine if it's FromAccount or ToAccount, we'll use a filter expression
		filterExpressions = append(filterExpressions, "(#fromAccount = :accountID OR #toAccount = :accountID)")
		expressionAttributeNames["#fromAccount"] = "FromAccount"
		expressionAttributeNames["#toAccount"] = "ToAccount"
		expressionAttributeValues[":accountID"] = &types.AttributeValueMemberS{Value: filter.AccountID}
	}

	if filter.StartTime != 0 && filter.EndTime != 0 {
		indexName = aws.String("TransactionDateIndex")
		keyConditionExpression += " AND #transactionDate BETWEEN :startTime AND :endTime"
		expressionAttributeNames["#transactionDate"] = "TransactionDate"
		expressionAttributeValues[":startTime"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(filter.StartTime, 10)}
		expressionAttributeValues[":endTime"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(filter.EndTime, 10)}
	}

	if filter.TransactionStatus != nil {
		filterExpressions = append(filterExpressions, "#transactionStatus = :transactionStatus")
		expressionAttributeNames["#transactionStatus"] = "TransactionStatus"
		expressionAttributeValues[":transactionStatus"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*filter.TransactionStatus)}
	}

	if filter.Limit == 0 {
		filter.Limit = 25
	}

	queryInput := &dynamodb.QueryInput{
		TableName:                 aws.String("TransactionsTable"),
		IndexName:                 indexName,
		KeyConditionExpression:    aws.String(keyConditionExpression),
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		Limit:                     aws.Int32(filter.Limit),
		ScanIndexForward:          aws.Bool(false), // To get the most recent transactions first
	}

	if len(filterExpressions) > 0 {
		queryInput.FilterExpression = aws.String(strings.Join(filterExpressions, " AND "))
	}

	if len(filter.LastEvaluatedKey) > 0 {
		queryInput.ExclusiveStartKey = filter.LastEvaluatedKey
	}

	// Debug: Print the query input
	fmt.Printf("Query Input: %+v\n", queryInput)

	output, err := dbSvc.Query(ctx, queryInput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch transactions: %v", err)
	}

	// Debug: Print the number of items returned
	fmt.Printf("Number of items returned: %d\n", len(output.Items))

	var transactions []TransactionEntry
	err = attributevalue.UnmarshalListOfMaps(output.Items, &transactions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal transactions: %v", err)
	}

	return transactions, output.LastEvaluatedKey, nil
}

// Helper function to append filter expressions
func addFilterExpression(existing, add string) string {
	if existing != "" {
		return existing + " AND " + add
	}
	return add
}

// Helper function to get the current timestamp
func getCurrentTimestamp() int64 {
	// Get the current time in UTC
	currentTime := time.Now().UTC()

	// Get the Unix timestamp (number of seconds since January 1, 1970)
	timestamp := currentTime.Unix()

	return timestamp
}
