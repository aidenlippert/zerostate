use crate as pallet_escrow;
use frame_support::{
    derive_impl, parameter_types,
    traits::{ConstU128, ConstU32},
};
use sp_runtime::BuildStorage;

type Block = frame_system::mocking::MockBlock<Test>;

// Configure a mock runtime to test the pallet.
frame_support::construct_runtime!(
    pub enum Test
    {
        System: frame_system,
        Balances: pallet_balances,
        Did: pallet_did,
        Registry: pallet_registry,
        Escrow: pallet_escrow,
    }
);

#[derive_impl(frame_system::config_preludes::TestDefaultConfig)]
impl frame_system::Config for Test {
    type Block = Block;
    type AccountData = pallet_balances::AccountData<u128>;
}

impl pallet_balances::Config for Test {
    type MaxLocks = ConstU32<50>;
    type MaxReserves = ();
    type ReserveIdentifier = [u8; 8];
    type Balance = u128;
    type RuntimeEvent = RuntimeEvent;
    type DustRemoval = ();
    type ExistentialDeposit = ConstU128<1>;
    type AccountStore = System;
    type WeightInfo = pallet_balances::weights::SubstrateWeight<Test>;
    type FreezeIdentifier = ();
    type MaxFreezes = ();
    type RuntimeHoldReason = ();
    type RuntimeFreezeReason = ();
    type DoneSlashHandler = ();
}

impl pallet_did::Config for Test {
    type RuntimeEvent = RuntimeEvent;
    type MaxDidLength = ConstU32<128>;
}

impl pallet_registry::Config for Test {
    type RuntimeEvent = RuntimeEvent;
    type MaxCapabilities = ConstU32<20>;
    type MaxNameLength = ConstU32<64>;
    type MaxCapabilityLength = ConstU32<64>;
}

parameter_types! {
    pub const DefaultTimeout: u64 = 1000; // 1000 blocks
    pub const ProtocolFeeAccount: u64 = 999; // Protocol fee account
    pub const MaxEscrowAmount: u128 = 1_000_000; // Maximum escrow amount
    pub const MaxParticipants: u32 = 10; // Maximum participants in multi-party escrow
    pub const MaxMilestones: u32 = 20; // Maximum milestones per escrow
    pub const MaxBatchSize: u32 = 50; // Maximum batch size for operations
}

impl pallet_escrow::Config for Test {
    type RuntimeEvent = RuntimeEvent;
    type Currency = Balances;
    type DefaultTimeout = DefaultTimeout;
    type ProtocolFeeAccount = ProtocolFeeAccount;
    type MaxEscrowAmount = MaxEscrowAmount;
    type MaxParticipants = MaxParticipants;
    type MaxMilestones = MaxMilestones;
    type MaxBatchSize = MaxBatchSize;
}

// Build genesis storage according to the mock runtime.
pub fn new_test_ext() -> sp_io::TestExternalities {
    let mut t = frame_system::GenesisConfig::<Test>::default()
        .build_storage()
        .unwrap();

    pallet_balances::GenesisConfig::<Test> {
        balances: vec![
            (1, 10000), // ALICE
            (2, 10000), // BOB
            (3, 10000), // CHARLIE
            (4, 10000), // DAVE
            (5, 10000), // EVE
            (999, 0),   // Protocol fee account
        ],
        dev_accounts: None,
    }
    .assimilate_storage(&mut t)
    .unwrap();

    t.into()
}
