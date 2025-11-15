package substrate

import (
	"time"
)

// =============================================================================
// MULTI-PARTY ESCROW TYPES
// =============================================================================

// ParticipantRole defines the role of a participant in multi-party escrow
type ParticipantRole uint8

const (
	ParticipantRoleClient       ParticipantRole = 0
	ParticipantRoleAgent        ParticipantRole = 1
	ParticipantRoleValidator    ParticipantRole = 2
	ParticipantRoleArbitrator   ParticipantRole = 3
	ParticipantRoleFeeCollector ParticipantRole = 4
	ParticipantRoleInsurer      ParticipantRole = 5
)

// String returns the string representation of ParticipantRole
func (pr ParticipantRole) String() string {
	switch pr {
	case ParticipantRoleClient:
		return "Client"
	case ParticipantRoleAgent:
		return "Agent"
	case ParticipantRoleValidator:
		return "Validator"
	case ParticipantRoleArbitrator:
		return "Arbitrator"
	case ParticipantRoleFeeCollector:
		return "FeeCollector"
	case ParticipantRoleInsurer:
		return "Insurer"
	default:
		return "Unknown"
	}
}

// EscrowParticipant represents a participant in a multi-party escrow
type EscrowParticipant struct {
	Account       AccountID       `json:"account"`        // Blockchain account ID
	DID           DID             `json:"did"`            // Decentralized identifier
	Role          ParticipantRole `json:"role"`           // Role in the escrow
	Amount        Balance         `json:"amount"`         // Amount contributed/allocated
	RequiredVotes uint32          `json:"required_votes"` // Votes required from this participant
	HasApproved   bool            `json:"has_approved"`   // Whether participant has approved
	JoinedAt      BlockNumber     `json:"joined_at"`      // When participant joined
}

// MultiPartyEscrowInfo represents multi-party escrow metadata
type MultiPartyEscrowInfo struct {
	TotalParticipants  uint32              `json:"total_participants"`   // Number of participants
	RequiredApprovals  uint32              `json:"required_approvals"`   // Minimum approvals needed
	CurrentApprovals   uint32              `json:"current_approvals"`    // Current approval count
	ApprovalThreshold  uint32              `json:"approval_threshold"`   // Percentage threshold (0-100)
	Participants       []EscrowParticipant `json:"participants"`         // List of participants
	IsApprovalComplete bool                `json:"is_approval_complete"` // Whether approval phase is complete
}

// =============================================================================
// MILESTONE ESCROW TYPES
// =============================================================================

// MilestoneStatus defines the status of a milestone
type MilestoneStatus uint8

const (
	MilestoneStatusPending    MilestoneStatus = 0
	MilestoneStatusInProgress MilestoneStatus = 1
	MilestoneStatusCompleted  MilestoneStatus = 2
	MilestoneStatusApproved   MilestoneStatus = 3
	MilestoneStatusRejected   MilestoneStatus = 4
	MilestoneStatusDisputed   MilestoneStatus = 5
)

// String returns the string representation of MilestoneStatus
func (ms MilestoneStatus) String() string {
	switch ms {
	case MilestoneStatusPending:
		return "Pending"
	case MilestoneStatusInProgress:
		return "InProgress"
	case MilestoneStatusCompleted:
		return "Completed"
	case MilestoneStatusApproved:
		return "Approved"
	case MilestoneStatusRejected:
		return "Rejected"
	case MilestoneStatusDisputed:
		return "Disputed"
	default:
		return "Unknown"
	}
}

// Milestone represents a single milestone in milestone-based escrow
type Milestone struct {
	ID                uint32          `json:"id"`                 // Unique milestone ID
	Description       string          `json:"description"`        // Milestone description
	Amount            Balance         `json:"amount"`             // Amount allocated to this milestone
	RequiredApprovals uint32          `json:"required_approvals"` // Number of approvals needed
	CurrentApprovals  uint32          `json:"current_approvals"`  // Current approval count
	Status            MilestoneStatus `json:"status"`             // Current milestone status
	CreatedAt         BlockNumber     `json:"created_at"`         // When milestone was created
	CompletedAt       *BlockNumber    `json:"completed_at"`       // When milestone was completed
	ApprovedAt        *BlockNumber    `json:"approved_at"`        // When milestone was approved
	DueDate           *BlockNumber    `json:"due_date"`           // Optional due date
	Dependencies      []uint32        `json:"dependencies"`       // IDs of prerequisite milestones
	ApprovalAccounts  []AccountID     `json:"approval_accounts"`  // Accounts that can approve
	Evidence          []string        `json:"evidence"`           // Evidence URLs/hashes
}

// MilestoneEscrowInfo represents milestone-based escrow metadata
type MilestoneEscrowInfo struct {
	TotalMilestones     uint32      `json:"total_milestones"`     // Total number of milestones
	CompletedMilestones uint32      `json:"completed_milestones"` // Number of completed milestones
	ApprovedMilestones  uint32      `json:"approved_milestones"`  // Number of approved milestones
	TotalAllocated      Balance     `json:"total_allocated"`      // Total amount allocated across milestones
	TotalReleased       Balance     `json:"total_released"`       // Total amount released so far
	Milestones          []Milestone `json:"milestones"`           // List of milestones
	AutoApprovalDelay   *uint32     `json:"auto_approval_delay"`  // Blocks to wait for auto-approval
}

// =============================================================================
// BATCH OPERATION TYPES
// =============================================================================

// BatchCreateEscrowRequest represents a single escrow creation request in a batch
type BatchCreateEscrowRequest struct {
	TaskID         [32]byte               `json:"task_id"`          // Unique task identifier
	Amount         uint64                 `json:"amount"`           // Escrow amount in smallest units
	TaskHash       [32]byte               `json:"task_hash"`        // Hash of task details
	TimeoutBlocks  *uint32                `json:"timeout_blocks"`   // Optional timeout in blocks
	EscrowType     EscrowType             `json:"escrow_type"`      // Type of escrow (simple, multi-party, milestone)
	MultiPartyInfo *MultiPartyEscrowInfo  `json:"multi_party_info"` // Multi-party specific data
	MilestoneInfo  *MilestoneEscrowInfo   `json:"milestone_info"`   // Milestone specific data
	RefundPolicy   *RefundPolicy          `json:"refund_policy"`    // Refund policy configuration
	TemplateID     *uint32                `json:"template_id"`      // Optional template ID
	CustomMetadata map[string]interface{} `json:"custom_metadata"`  // Custom metadata
}

// BatchCreateEscrowResult represents the result of a batch escrow creation
type BatchCreateEscrowResult struct {
	SuccessfulTasks []BatchEscrowTaskResult `json:"successful_tasks"` // Successfully created escrows
	FailedTasks     []BatchEscrowTaskError  `json:"failed_tasks"`     // Failed escrow creations
	TotalProcessed  uint32                  `json:"total_processed"`  // Total number of requests processed
	TotalSucceeded  uint32                  `json:"total_succeeded"`  // Number of successful creations
	TotalFailed     uint32                  `json:"total_failed"`     // Number of failed creations
	TransactionHash Hash                    `json:"transaction_hash"` // Transaction hash for the batch
	BlockNumber     BlockNumber             `json:"block_number"`     // Block number where batch was processed
}

// BatchEscrowTaskResult represents a successful escrow creation in a batch
type BatchEscrowTaskResult struct {
	TaskID    [32]byte    `json:"task_id"`    // Task identifier
	EscrowID  [32]byte    `json:"escrow_id"`  // Generated escrow identifier
	Amount    Balance     `json:"amount"`     // Escrow amount
	CreatedAt BlockNumber `json:"created_at"` // Creation block
}

// BatchEscrowTaskError represents a failed escrow creation in a batch
type BatchEscrowTaskError struct {
	TaskID    [32]byte `json:"task_id"`    // Task identifier
	Error     string   `json:"error"`      // Error message
	ErrorCode uint32   `json:"error_code"` // Error code for categorization
}

// =============================================================================
// REFUND POLICY TYPES
// =============================================================================

// RefundPolicyType defines the type of refund policy
type RefundPolicyType uint8

const (
	RefundPolicyTypeNone        RefundPolicyType = 0 // No automatic refunds
	RefundPolicyTypeLinear      RefundPolicyType = 1 // Linear decrease over time
	RefundPolicyTypeExponential RefundPolicyType = 2 // Exponential decrease over time
	RefundPolicyTypeStepwise    RefundPolicyType = 3 // Step-wise decrease at intervals
	RefundPolicyTypeFixed       RefundPolicyType = 4 // Fixed refund amount regardless of time
	RefundPolicyTypeCustom      RefundPolicyType = 5 // Custom logic (external calculation)
)

// String returns the string representation of RefundPolicyType
func (rpt RefundPolicyType) String() string {
	switch rpt {
	case RefundPolicyTypeNone:
		return "None"
	case RefundPolicyTypeLinear:
		return "Linear"
	case RefundPolicyTypeExponential:
		return "Exponential"
	case RefundPolicyTypeStepwise:
		return "Stepwise"
	case RefundPolicyTypeFixed:
		return "Fixed"
	case RefundPolicyTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// RefundStep represents a step in a stepwise refund policy
type RefundStep struct {
	Threshold        uint32 `json:"threshold"`         // Time threshold (blocks or percentage)
	RefundPercentage uint32 `json:"refund_percentage"` // Refund percentage at this step (0-100)
}

// RefundPolicy defines the refund policy for an escrow
type RefundPolicy struct {
	PolicyType       RefundPolicyType       `json:"policy_type"`       // Type of refund policy
	InitialRefund    uint32                 `json:"initial_refund"`    // Initial refund percentage (0-100)
	MinimumRefund    uint32                 `json:"minimum_refund"`    // Minimum refund percentage (0-100)
	MaximumRefund    uint32                 `json:"maximum_refund"`    // Maximum refund percentage (0-100)
	DecayRate        *uint32                `json:"decay_rate"`        // Decay rate for exponential policies
	StepInterval     *uint32                `json:"step_interval"`     // Interval for stepwise policies (blocks)
	Steps            []RefundStep           `json:"steps"`             // Steps for stepwise policies
	GracePeriod      *uint32                `json:"grace_period"`      // Grace period in blocks
	PenaltyRate      *uint32                `json:"penalty_rate"`      // Penalty rate for early refunds
	CustomParameters map[string]interface{} `json:"custom_parameters"` // Custom parameters for complex policies
}

// RefundCalculation represents the result of a refund calculation
type RefundCalculation struct {
	OriginalAmount   Balance          `json:"original_amount"`   // Original escrow amount
	RefundAmount     Balance          `json:"refund_amount"`     // Calculated refund amount
	PenaltyAmount    Balance          `json:"penalty_amount"`    // Penalty amount deducted
	RefundPercentage uint32           `json:"refund_percentage"` // Actual refund percentage
	CalculatedAt     BlockNumber      `json:"calculated_at"`     // Block when calculation was done
	ExpiresAt        BlockNumber      `json:"expires_at"`        // When this calculation expires
	PolicyType       RefundPolicyType `json:"policy_type"`       // Policy type used
	Reason           string           `json:"reason"`            // Reason for the refund calculation
}

// =============================================================================
// TEMPLATE TYPES
// =============================================================================

// EscrowTemplateType defines the type of escrow template
type EscrowTemplateType uint8

const (
	EscrowTemplateTypeSimple       EscrowTemplateType = 0
	EscrowTemplateTypeMultiParty   EscrowTemplateType = 1
	EscrowTemplateTypeMilestone    EscrowTemplateType = 2
	EscrowTemplateTypeSubscription EscrowTemplateType = 3
	EscrowTemplateTypeInsurance    EscrowTemplateType = 4
	EscrowTemplateTypeCustom       EscrowTemplateType = 5
)

// String returns the string representation of EscrowTemplateType
func (ett EscrowTemplateType) String() string {
	switch ett {
	case EscrowTemplateTypeSimple:
		return "Simple"
	case EscrowTemplateTypeMultiParty:
		return "MultiParty"
	case EscrowTemplateTypeMilestone:
		return "Milestone"
	case EscrowTemplateTypeSubscription:
		return "Subscription"
	case EscrowTemplateTypeInsurance:
		return "Insurance"
	case EscrowTemplateTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// EscrowTemplate represents a reusable escrow configuration template
type EscrowTemplate struct {
	ID           uint32             `json:"id"`            // Unique template identifier
	Name         string             `json:"name"`          // Human-readable template name
	Description  string             `json:"description"`   // Template description
	TemplateType EscrowTemplateType `json:"template_type"` // Type of template
	Creator      AccountID          `json:"creator"`       // Account that created the template
	IsPublic     bool               `json:"is_public"`     // Whether template is publicly usable
	Version      uint32             `json:"version"`       // Template version
	Tags         []string           `json:"tags"`          // Searchable tags

	// Default configurations
	DefaultTimeout    *uint32 `json:"default_timeout"`     // Default timeout in blocks
	DefaultFeePercent *uint8  `json:"default_fee_percent"` // Default fee percentage
	MinAmount         *uint64 `json:"min_amount"`          // Minimum escrow amount
	MaxAmount         *uint64 `json:"max_amount"`          // Maximum escrow amount

	// Template-specific configurations
	MultiPartyTemplate   *MultiPartyTemplate `json:"multi_party_template"`   // Multi-party specific config
	MilestoneTemplate    *MilestoneTemplate  `json:"milestone_template"`     // Milestone specific config
	RefundPolicyTemplate *RefundPolicy       `json:"refund_policy_template"` // Default refund policy

	// Metadata and constraints
	RequiredFields       []string `json:"required_fields"`       // Required fields when using template
	AllowedModifications []string `json:"allowed_modifications"` // Fields that can be modified
	CustomValidation     *string  `json:"custom_validation"`     // Custom validation rules (JSON)

	// Usage statistics and management
	UsageCount uint64       `json:"usage_count"` // Number of times used
	CreatedAt  BlockNumber  `json:"created_at"`  // When template was created
	UpdatedAt  BlockNumber  `json:"updated_at"`  // When template was last updated
	IsActive   bool         `json:"is_active"`   // Whether template is active
	ExpiresAt  *BlockNumber `json:"expires_at"`  // Optional expiration
}

// MultiPartyTemplate represents default multi-party configuration for templates
type MultiPartyTemplate struct {
	MinParticipants  uint32              `json:"min_participants"`  // Minimum number of participants
	MaxParticipants  uint32              `json:"max_participants"`  // Maximum number of participants
	DefaultThreshold uint32              `json:"default_threshold"` // Default approval threshold (percentage)
	AllowedRoles     []ParticipantRole   `json:"allowed_roles"`     // Roles allowed in this template
	RequiredRoles    []ParticipantRole   `json:"required_roles"`    // Roles required in this template
	RolePermissions  map[string][]string `json:"role_permissions"`  // Permissions per role
}

// MilestoneTemplate represents default milestone configuration for templates
type MilestoneTemplate struct {
	MinMilestones       uint32   `json:"min_milestones"`        // Minimum number of milestones
	MaxMilestones       uint32   `json:"max_milestones"`        // Maximum number of milestones
	DefaultApprovers    uint32   `json:"default_approvers"`     // Default number of required approvers
	AllowDependencies   bool     `json:"allow_dependencies"`    // Whether milestone dependencies are allowed
	RequiredEvidence    []string `json:"required_evidence"`     // Types of evidence required
	AutoApprovalDefault *uint32  `json:"auto_approval_default"` // Default auto-approval delay
}

// TemplateUsage represents usage statistics for a template
type TemplateUsage struct {
	TemplateID    uint32      `json:"template_id"`   // Template identifier
	User          AccountID   `json:"user"`          // User who used the template
	TaskID        [32]byte    `json:"task_id"`       // Task ID where template was used
	UsedAt        BlockNumber `json:"used_at"`       // When template was used
	Modifications []string    `json:"modifications"` // Fields that were modified from template
	Success       bool        `json:"success"`       // Whether usage was successful
}

// =============================================================================
// ESCROW TYPE EXTENSIONS
// =============================================================================

// EscrowType defines the type of escrow (extension of existing types)
type EscrowType uint8

const (
	EscrowTypeSimple     EscrowType = 0
	EscrowTypeMultiParty EscrowType = 1
	EscrowTypeMilestone  EscrowType = 2
	EscrowTypeHybrid     EscrowType = 3 // Combination of multi-party and milestone
)

// String returns the string representation of EscrowType
func (et EscrowType) String() string {
	switch et {
	case EscrowTypeSimple:
		return "Simple"
	case EscrowTypeMultiParty:
		return "MultiParty"
	case EscrowTypeMilestone:
		return "Milestone"
	case EscrowTypeHybrid:
		return "Hybrid"
	default:
		return "Unknown"
	}
}

// ExtendedEscrowDetails extends the existing EscrowDetails with new functionality
type ExtendedEscrowDetails struct {
	// Base escrow information (from existing EscrowDetails)
	TaskID       [32]byte    `json:"task_id"`
	User         AccountID   `json:"user"`
	AgentDID     *DID        `json:"agent_did,omitempty"`
	AgentAccount *AccountID  `json:"agent_account,omitempty"`
	Amount       Balance     `json:"amount"`
	FeePercent   uint8       `json:"fee_percent"`
	CreatedAt    BlockNumber `json:"created_at"`
	ExpiresAt    BlockNumber `json:"expires_at"`
	State        EscrowState `json:"state"`
	TaskHash     [32]byte    `json:"task_hash"`

	// Extended information
	EscrowType        EscrowType             `json:"escrow_type"`        // Type of escrow
	MultiPartyInfo    *MultiPartyEscrowInfo  `json:"multi_party_info"`   // Multi-party specific data
	MilestoneInfo     *MilestoneEscrowInfo   `json:"milestone_info"`     // Milestone specific data
	RefundPolicy      *RefundPolicy          `json:"refund_policy"`      // Refund policy
	TemplateID        *uint32                `json:"template_id"`        // Template used (if any)
	CustomMetadata    map[string]interface{} `json:"custom_metadata"`    // Custom metadata
	LastUpdated       BlockNumber            `json:"last_updated"`       // Last modification block
	TotalFees         Balance                `json:"total_fees"`         // Total fees collected
	InsuranceCoverage *Balance               `json:"insurance_coverage"` // Insurance coverage amount
}

// =============================================================================
// UTILITY TYPES
// =============================================================================

// EscrowFilter represents filtering options for escrow queries
type EscrowFilter struct {
	User          *AccountID   `json:"user"`           // Filter by user account
	Agent         *AccountID   `json:"agent"`          // Filter by agent account
	State         *EscrowState `json:"state"`          // Filter by escrow state
	EscrowType    *EscrowType  `json:"escrow_type"`    // Filter by escrow type
	MinAmount     *uint64      `json:"min_amount"`     // Minimum amount filter
	MaxAmount     *uint64      `json:"max_amount"`     // Maximum amount filter
	CreatedAfter  *BlockNumber `json:"created_after"`  // Created after block
	CreatedBefore *BlockNumber `json:"created_before"` // Created before block
	ExpiresAfter  *BlockNumber `json:"expires_after"`  // Expires after block
	ExpiresBefore *BlockNumber `json:"expires_before"` // Expires before block
	TemplateID    *uint32      `json:"template_id"`    // Filter by template
	HasMilestones *bool        `json:"has_milestones"` // Whether escrow has milestones
	IsMultiParty  *bool        `json:"is_multi_party"` // Whether escrow is multi-party
}

// EscrowStats represents statistics about escrows in the system
type EscrowStats struct {
	TotalEscrows            uint64        `json:"total_escrows"`              // Total number of escrows
	ActiveEscrows           uint64        `json:"active_escrows"`             // Number of active escrows
	CompletedEscrows        uint64        `json:"completed_escrows"`          // Number of completed escrows
	DisputedEscrows         uint64        `json:"disputed_escrows"`           // Number of disputed escrows
	TotalValueLocked        Balance       `json:"total_value_locked"`         // Total value locked in escrows
	TotalFeesCollected      Balance       `json:"total_fees_collected"`       // Total fees collected
	AverageEscrowAmount     Balance       `json:"average_escrow_amount"`      // Average escrow amount
	AverageTimeToCompletion time.Duration `json:"average_time_to_completion"` // Average completion time
	SuccessRate             float64       `json:"success_rate"`               // Success rate (0-1)

	// Type-specific stats
	SimpleEscrows     uint64 `json:"simple_escrows"`      // Number of simple escrows
	MultiPartyEscrows uint64 `json:"multi_party_escrows"` // Number of multi-party escrows
	MilestoneEscrows  uint64 `json:"milestone_escrows"`   // Number of milestone escrows
	HybridEscrows     uint64 `json:"hybrid_escrows"`      // Number of hybrid escrows

	LastUpdated BlockNumber `json:"last_updated"` // When stats were last calculated
}

// =============================================================================
// EVENT TYPES
// =============================================================================

// EscrowEvent represents events emitted by the escrow system
type EscrowEvent struct {
	EventType   string                 `json:"event_type"`   // Type of event
	TaskID      [32]byte               `json:"task_id"`      // Associated task ID
	BlockNumber BlockNumber            `json:"block_number"` // Block where event occurred
	TxHash      Hash                   `json:"tx_hash"`      // Transaction hash
	Timestamp   time.Time              `json:"timestamp"`    // Event timestamp
	Data        map[string]interface{} `json:"data"`         // Event-specific data
}

// Specific event types for type-safe event handling
type (
	// Multi-party escrow events
	ParticipantAddedEvent struct {
		TaskID      [32]byte          `json:"task_id"`
		Participant EscrowParticipant `json:"participant"`
		AddedBy     AccountID         `json:"added_by"`
	}

	ParticipantRemovedEvent struct {
		TaskID             [32]byte  `json:"task_id"`
		ParticipantAccount AccountID `json:"participant_account"`
		RemovedBy          AccountID `json:"removed_by"`
		Reason             string    `json:"reason"`
	}

	MultiPartyApprovedEvent struct {
		TaskID            [32]byte `json:"task_id"`
		TotalApprovals    uint32   `json:"total_approvals"`
		RequiredApprovals uint32   `json:"required_approvals"`
	}

	// Milestone events
	MilestoneAddedEvent struct {
		TaskID    [32]byte  `json:"task_id"`
		Milestone Milestone `json:"milestone"`
		AddedBy   AccountID `json:"added_by"`
	}

	MilestoneCompletedEvent struct {
		TaskID      [32]byte  `json:"task_id"`
		MilestoneID uint32    `json:"milestone_id"`
		CompletedBy AccountID `json:"completed_by"`
		Evidence    []string  `json:"evidence"`
	}

	MilestoneApprovedEvent struct {
		TaskID      [32]byte  `json:"task_id"`
		MilestoneID uint32    `json:"milestone_id"`
		ApprovedBy  AccountID `json:"approved_by"`
		Amount      Balance   `json:"amount"`
	}

	// Refund policy events
	RefundPolicySetEvent struct {
		TaskID       [32]byte     `json:"task_id"`
		RefundPolicy RefundPolicy `json:"refund_policy"`
		SetBy        AccountID    `json:"set_by"`
	}

	RefundCalculatedEvent struct {
		TaskID            [32]byte          `json:"task_id"`
		RefundCalculation RefundCalculation `json:"refund_calculation"`
		CalculatedBy      AccountID         `json:"calculated_by"`
	}

	// Template events
	TemplateCreatedEvent struct {
		TemplateID uint32         `json:"template_id"`
		Template   EscrowTemplate `json:"template"`
		CreatedBy  AccountID      `json:"created_by"`
	}

	TemplateUsedEvent struct {
		TemplateID    uint32    `json:"template_id"`
		TaskID        [32]byte  `json:"task_id"`
		UsedBy        AccountID `json:"used_by"`
		Modifications []string  `json:"modifications"`
	}
)
