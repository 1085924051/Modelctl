package ui

type capabilityTemplate struct {
	Name        string
	ID          string
	Version     string
	DisplayName string
	Description string
	Fields      string
	Input       string
	Questions   []capabilityTemplateQuestion
}

type capabilityTemplateQuestion struct {
	ID           string
	Type         string
	Instructions string
	Criteria     string
}

func capabilityTemplates() []capabilityTemplate {
	return []capabilityTemplate{
		{
			Name:        "Refund request detection",
			ID:          "refund-check",
			Version:     "1.0.0",
			DisplayName: "Refund request check",
			Description: "Decide whether a customer is explicitly asking for a refund.",
			Fields:      "text:string:Customer message",
			Input:       "Customer message: {{text}}",
			Questions: []capabilityTemplateQuestion{{
				ID: "refund", Type: "noul", Instructions: "Does the customer explicitly ask for a refund?",
			}},
		},
		{
			Name:        "Support ticket routing",
			ID:          "ticket-routing",
			Version:     "1.0.0",
			DisplayName: "Support ticket routing",
			Description: "Route an incoming support ticket to the team that can resolve it.",
			Fields:      "text:string:Ticket message\ncustomer_tier?:string:Optional customer tier",
			Input:       "Ticket: {{text}}\nCustomer tier: {{customer_tier}}",
			Questions: []capabilityTemplateQuestion{{
				ID: "team", Type: "choice", Instructions: "Which team should handle this ticket?", Criteria: "billing: Billing and payments\nsupport: Product support\naccount: Account access",
			}, {
				ID: "priority", Type: "score", Instructions: "How urgent is this ticket?", Criteria: "low\nmedium\nhigh",
			}},
		},
		{
			Name:        "Document risk triage",
			ID:          "document-risk",
			Version:     "1.0.0",
			DisplayName: "Document risk triage",
			Description: "Identify whether a document needs a human review before it is approved.",
			Fields:      "document:string:Document text\ndocument_type?:string:Optional document type",
			Input:       "Document type: {{document_type}}\nDocument text: {{document}}",
			Questions: []capabilityTemplateQuestion{{
				ID: "review", Type: "noul", Instructions: "Does this document require human review before approval?",
			}, {
				ID: "risk", Type: "choice", Instructions: "What is the document's risk level?", Criteria: "low: Low risk\nmedium: Medium risk\nhigh: High risk",
			}},
		},
	}
}
