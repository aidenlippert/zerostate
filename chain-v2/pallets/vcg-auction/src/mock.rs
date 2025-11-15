use crate as pallet_vcg_auction;
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
        VcgAuction: pallet_vcg_auction,
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
    pub const MaxBidsPerAuction: u32 = 100;
    pub const DefaultAuctionDuration: u64 = 100; // 100 blocks
    pub const MinimumBidAmount: u128 = 10;
}

impl pallet_vcg_auction::Config for Test {
    type RuntimeEvent = RuntimeEvent;
    type Balance = u128;
    type MaxBidsPerAuction = MaxBidsPerAuction;
    type DefaultAuctionDuration = DefaultAuctionDuration;
    type MinimumBidAmount = MinimumBidAmount;
}

// Build genesis storage according to the mock runtime.
pub fn new_test_ext() -> sp_io::TestExternalities {
    let mut t = frame_system::GenesisConfig::<Test>::default()
        .build_storage()
        .unwrap();

    pallet_balances::GenesisConfig::<Test> {
        balances: vec![(1, 10000), (2, 10000), (3, 10000), (4, 10000)],
        dev_accounts: None,
    }
    .assimilate_storage(&mut t)
    .unwrap();

    t.into()
}
