type TemplateMilestoneConfig<T> = (Vec<u8>, BalanceOf<T>, u32);
type TemplateParticipantConfig<T> = (
    <T as frame_system::Config>::AccountId,
    ParticipantRole,
    BalanceOf<T>,
);

// # Sprint 8 Phase 2: Escrow Template System
//
// This module implements a template system for creating standardized escrow contracts.
// Templates provide pre-configured escrow patterns for common use cases.

use codec::DecodeWithMemTracking;
use frame_support::pallet_prelude::*;
use frame_system::pallet_prelude::*;
use sp_std::vec::Vec;

use super::*;

/// Template types for different escrow use cases
#[derive(
    Clone,
    Encode,
    Decode,
    DecodeWithMemTracking,
    Eq,
    PartialEq,
    RuntimeDebug,
    TypeInfo,
    MaxEncodedLen,
)]
pub enum TemplateType {
    /// Basic one-to-one payment escrow
    SimplePayment,
    /// Project with multiple milestones and deliverables
    MilestoneProject,
    /// Contract involving multiple parties with different roles
    MultiPartyContract,
    /// Payment that releases after a specific time period
    TimeLockedRelease,
    /// Payment conditional on external factors or approvals
    ConditionalPayment,
    /// Purchase agreement with buyer, seller, and optional arbiter
    EscrowedPurchase,
    /// Recurring subscription-based payments
    SubscriptionPayment,
    /// Custom user-defined template
    Custom,
}

/// Template parameters configuration
#[derive(
    Clone,
    Encode,
    Decode,
    DecodeWithMemTracking,
    Eq,
    PartialEq,
    RuntimeDebug,
    TypeInfo,
    MaxEncodedLen,
)]
#[scale_info(skip_type_params(T))]
pub struct TemplateParams<T: Config> {
    /// Default timeout in blocks
    pub default_timeout: Option<BlockNumberFor<T>>,
    /// Default fee percentage (0-100)
    pub default_fee_percent: Option<u8>,
    /// Whether multi-party support is enabled
    pub multi_party_enabled: bool,
    /// Whether milestone support is enabled
    pub milestone_enabled: bool,
    /// Maximum participants allowed
    pub max_participants: Option<u32>,
    /// Maximum milestones allowed
    pub max_milestones: Option<u32>,
    /// Default required approvals for milestones
    pub default_milestone_approvals: Option<u32>,
    /// Minimum escrow amount
    pub min_amount: Option<BalanceOf<T>>,
    /// Maximum escrow amount
    pub max_amount: Option<BalanceOf<T>>,
    /// Auto-accept timeout in blocks
    pub auto_accept_timeout: Option<BlockNumberFor<T>>,
    /// Auto-release timeout in blocks
    pub auto_release_timeout: Option<BlockNumberFor<T>>,
    /// Whether disputes are allowed
    pub disputes_enabled: bool,
}

impl<T: Config> Default for TemplateParams<T> {
    fn default() -> Self {
        Self {
            default_timeout: None,
            default_fee_percent: Some(5),
            multi_party_enabled: false,
            milestone_enabled: false,
            max_participants: None,
            max_milestones: None,
            default_milestone_approvals: Some(1),
            min_amount: None,
            max_amount: None,
            auto_accept_timeout: None,
            auto_release_timeout: None,
            disputes_enabled: true,
        }
    }
}

/// Escrow template definition
#[derive(
    Clone,
    Encode,
    Decode,
    DecodeWithMemTracking,
    Eq,
    PartialEq,
    RuntimeDebug,
    TypeInfo,
    MaxEncodedLen,
)]
#[scale_info(skip_type_params(T))]
pub struct EscrowTemplate<T: Config> {
    /// Unique template identifier
    pub template_id: u32,
    /// Human-readable template name
    pub name: BoundedVec<u8, ConstU32<128>>,
    /// Template description
    pub description: BoundedVec<u8, ConstU32<512>>,
    /// Template type category
    pub template_type: TemplateType,
    /// Default parameters for escrows created from this template
    pub default_params: TemplateParams<T>,
    /// Whether the template is active and can be used
    pub is_active: bool,
    /// Account that created this template
    pub created_by: T::AccountId,
    /// Block when template was created
    pub created_at: BlockNumberFor<T>,
    /// Usage count for analytics
    pub usage_count: u32,
}

/// Built-in template definitions
impl<T: Config> EscrowTemplate<T> {
    /// Create a Simple Payment template
    pub fn simple_payment(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Simple Payment".to_vec().try_into().unwrap_or_default();
        let description =
            b"Basic one-to-one payment escrow for simple transactions between payer and payee."
                .to_vec()
                .try_into()
                .unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(5),
            multi_party_enabled: false,
            milestone_enabled: false,
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::SimplePayment,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Milestone Project template
    pub fn milestone_project(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Milestone Project".to_vec().try_into().unwrap_or_default();
        let description = b"Project-based escrow with multiple milestones and deliverables for complex work agreements.".to_vec().try_into().unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(3),
            multi_party_enabled: true,
            milestone_enabled: true,
            max_milestones: Some(10),
            default_milestone_approvals: Some(1),
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::MilestoneProject,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Multi-Party Contract template
    pub fn multi_party_contract(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Multi-Party Contract"
            .to_vec()
            .try_into()
            .unwrap_or_default();
        let description = b"Complex contract involving multiple stakeholders with different roles and responsibilities.".to_vec().try_into().unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(4),
            multi_party_enabled: true,
            milestone_enabled: true,
            max_participants: Some(10),
            max_milestones: Some(20),
            default_milestone_approvals: Some(2),
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::MultiPartyContract,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Time-Locked Release template
    pub fn time_locked_release(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Time-Locked Release"
            .to_vec()
            .try_into()
            .unwrap_or_default();
        let description = b"Payment that automatically releases after a specific time period without manual intervention.".to_vec().try_into().unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(2),
            multi_party_enabled: false,
            milestone_enabled: false,
            auto_release_timeout: Some(T::DefaultTimeout::get()),
            disputes_enabled: false,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::TimeLockedRelease,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Conditional Payment template
    pub fn conditional_payment(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Conditional Payment"
            .to_vec()
            .try_into()
            .unwrap_or_default();
        let description = b"Payment conditional on external factors, approvals, or specific conditions being met.".to_vec().try_into().unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(6),
            multi_party_enabled: true,
            milestone_enabled: true,
            max_participants: Some(5),
            default_milestone_approvals: Some(2),
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::ConditionalPayment,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create an Escrowed Purchase template
    pub fn escrowed_purchase(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Escrowed Purchase".to_vec().try_into().unwrap_or_default();
        let description = b"Secure purchase agreement between buyer and seller with optional arbiter for dispute resolution.".to_vec().try_into().unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(3),
            multi_party_enabled: true,
            milestone_enabled: false,
            max_participants: Some(3), // buyer, seller, arbiter
            default_milestone_approvals: Some(1),
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::EscrowedPurchase,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Subscription Payment template
    pub fn subscription_payment(
        template_id: u32,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Self {
        let name = b"Subscription Payment"
            .to_vec()
            .try_into()
            .unwrap_or_default();
        let description =
            b"Recurring payment system for subscription-based services with automated renewals."
                .to_vec()
                .try_into()
                .unwrap_or_default();

        let params = TemplateParams {
            default_fee_percent: Some(2),
            multi_party_enabled: false,
            milestone_enabled: true,  // for recurring periods
            max_milestones: Some(12), // monthly for a year
            default_milestone_approvals: Some(1),
            auto_release_timeout: Some(T::DefaultTimeout::get()),
            disputes_enabled: true,
            ..Default::default()
        };

        Self {
            template_id,
            name,
            description,
            template_type: TemplateType::SubscriptionPayment,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        }
    }

    /// Create a Custom template
    pub fn custom(
        template_id: u32,
        name: Vec<u8>,
        description: Vec<u8>,
        params: TemplateParams<T>,
        created_by: T::AccountId,
        created_at: BlockNumberFor<T>,
    ) -> Result<Self, DispatchError> {
        let bounded_name = name
            .try_into()
            .map_err(|_| Error::<T>::TemplateNameTooLong)?;
        let bounded_description = description
            .try_into()
            .map_err(|_| Error::<T>::TemplateDescriptionTooLong)?;

        Ok(Self {
            template_id,
            name: bounded_name,
            description: bounded_description,
            template_type: TemplateType::Custom,
            default_params: params,
            is_active: true,
            created_by,
            created_at,
            usage_count: 0,
        })
    }
}

/// Template configuration for creating escrows
#[derive(Clone, Encode, Decode, DecodeWithMemTracking, Eq, PartialEq, RuntimeDebug, TypeInfo)]
#[scale_info(skip_type_params(T))]
pub struct TemplateEscrowConfig<T: Config> {
    /// Template ID to use
    pub template_id: u32,
    /// Override default timeout (optional)
    pub timeout_override: Option<BlockNumberFor<T>>,
    /// Override default fee percentage (optional)
    pub fee_percent_override: Option<u8>,
    /// Override minimum amount (optional)
    pub min_amount_override: Option<BalanceOf<T>>,
    /// Override maximum amount (optional)
    pub max_amount_override: Option<BalanceOf<T>>,
    /// Additional milestone configurations for milestone-based templates
    pub milestone_configs: Option<Vec<TemplateMilestoneConfig<T>>>, // (description, amount, required_approvals)
    /// Additional participant configurations for multi-party templates
    pub participant_configs: Option<Vec<TemplateParticipantConfig<T>>>,
}

/// Template validation and utility functions
// Template helper functions are implemented in the main pallet impl block in lib.rs
/// Template-related errors
#[derive(Encode, Decode, Clone, PartialEq, Eq, RuntimeDebug, TypeInfo)]
pub enum TemplateError {
    TemplateNotFound,
    TemplateInactive,
    TemplateNameTooLong,
    TemplateDescriptionTooLong,
    InvalidTemplateParams,
    TooManyTemplates,
    InvalidFeePercentage,
    InvalidAmountRange,
}
