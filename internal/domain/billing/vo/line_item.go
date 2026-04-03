package vo

// LineItemType categorises a line item on an invoice.
type LineItemType string

const (
	LineItemPlan   LineItemType = "plan"
	LineItemAddon  LineItemType = "addon"
	LineItemCredit LineItemType = "credit"
)

// LineItem represents a single entry on an invoice.
type LineItem struct {
	Description string
	Type        LineItemType
	Amount      Money
	Quantity    int
}

// NewLineItem creates a LineItem with the given attributes.
func NewLineItem(desc string, itemType LineItemType, amount Money, quantity int) LineItem {
	return LineItem{
		Description: desc,
		Type:        itemType,
		Amount:      amount,
		Quantity:    quantity,
	}
}

// Total returns Amount multiplied by Quantity.
func (li LineItem) Total() Money {
	return li.Amount.Multiply(int64(li.Quantity))
}
