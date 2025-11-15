#!/usr/bin/env python3
"""
AINU Token Economic Simulator
Simulates token economics over time with various market scenarios
"""

import numpy as np
import matplotlib.pyplot as plt
from dataclasses import dataclass
from typing import List, Tuple
import json


@dataclass
class SimulationParams:
    """Parameters for economic simulation"""
    # Token supply
    total_supply: float = 10_000_000_000  # 10B AINU
    initial_circulating: float = 0.30  # 30% initially circulating
    
    # Burn mechanics
    burn_rate: float = 0.05  # 5% per transfer
    burn_enabled: bool = True
    
    # Staking
    staking_apr: float = 0.20  # 20% APR (increased from 12%)
    initial_staked_pct: float = 0.20  # 20% staked initially
    
    # Time-based multipliers (for lock durations)
    has_multipliers: bool = True
    multiplier_3mo: float = 1.0   # 3 months = 20% effective
    multiplier_6mo: float = 1.5   # 6 months = 30% effective
    multiplier_12mo: float = 2.0  # 12 months = 40% effective
    avg_multiplier: float = 1.5   # Weighted average (assuming 50% choose 6mo+)
    
    # Auto-compounding
    auto_compound_rate: float = 0.50  # 50% of stakers compound (up from 30%)
    
    # Revenue
    daily_tasks: int = 1000  # Tasks per day
    avg_task_fee: float = 10  # AINU per task
    
    # Splits
    agent_share: float = 0.70
    node_share: float = 0.20
    protocol_share: float = 0.05
    burn_share: float = 0.05
    
    # Growth
    task_growth_rate: float = 0.05  # 5% monthly growth
    
    # Simulation
    days: int = 365 * 2  # 2 years


class TokenEconomy:
    """Simulates AINU token economy"""
    
    def __init__(self, params: SimulationParams):
        self.params = params
        self.reset()
    
    def reset(self):
        """Reset simulation state"""
        self.day = 0
        self.circulating = self.params.total_supply * self.params.initial_circulating
        self.staked = self.params.total_supply * self.params.initial_staked_pct
        self.burned = 0
        self.treasury = 0
        
        # Historical data
        self.history = {
            'day': [],
            'circulating': [],
            'staked': [],
            'burned': [],
            'treasury': [],
            'daily_revenue': [],
            'burn_rate_actual': [],
            'staking_ratio': [],
            'price_index': []
        }
    
    def simulate_day(self) -> dict:
        """Simulate one day of activity"""
        # Calculate daily tasks (with growth)
        months_elapsed = self.day / 30
        growth_multiplier = (1 + self.params.task_growth_rate) ** months_elapsed
        daily_tasks = int(self.params.daily_tasks * growth_multiplier)
        
        # Calculate revenue
        daily_revenue = daily_tasks * self.params.avg_task_fee
        
        # Revenue distribution
        agent_rewards = daily_revenue * self.params.agent_share
        node_rewards = daily_revenue * self.params.node_share
        protocol_revenue = daily_revenue * self.params.protocol_share
        burn_amount = daily_revenue * self.params.burn_share
        
        # Process burns from fees
        self.burned += burn_amount
        self.circulating -= burn_amount
        
        # Add to treasury
        self.treasury += protocol_revenue
        
        # Simulate transfer burns (agents/nodes moving tokens)
        # Assume 50% of rewards are transferred (not staked)
        transferred = (agent_rewards + node_rewards) * 0.5
        if self.params.burn_enabled:
            transfer_burn = transferred * self.params.burn_rate
            self.burned += transfer_burn
            self.circulating -= transfer_burn
        
        # Staking dynamics
        # Some rewards get staked (increased from 30% to 50% with better 20-40% APR incentives)
        newly_staked = (agent_rewards + node_rewards) * 0.50
        self.staked += newly_staked
        self.circulating -= newly_staked
        
        # Some unstaking happens (reduced from 0.1% to 0.03% with better lock incentives)
        # Users lock for 6-12 months now, so much less daily unstaking
        unstake_rate = 0.0003  # 0.03% daily unstake rate
        unstaked = self.staked * unstake_rate
        self.staked -= unstaked
        self.circulating += unstaked
        
        # Staking rewards with multiplier
        # Base APR * avg_multiplier (1.5x) = effective APR (30%)
        effective_apr = self.params.staking_apr * self.params.avg_multiplier
        daily_staking_rewards = self.staked * (effective_apr / 365)
        
        # Auto-compounding (50% of stakers compound)
        compounded = daily_staking_rewards * self.params.auto_compound_rate
        claimed = daily_staking_rewards - compounded
        
        # Compounded rewards stay staked
        self.staked += compounded
        
        # Claimed rewards enter circulation (but many are re-staked due to good APR)
        # Assume 30% of claimed rewards get re-staked
        restaked = claimed * 0.30
        self.staked += restaked
        self.circulating += (claimed - restaked)
        
        # Calculate metrics
        staking_ratio = self.staked / self.params.total_supply
        actual_burn_rate = self.burned / self.params.total_supply
        
        # Simple price index (supply/demand dynamics)
        # Price goes up with: more staking, more burning, more usage
        # Price goes down with: more circulating supply
        price_factors = (
            (1 + staking_ratio * 2) *  # Staking reduces sell pressure
            (1 + actual_burn_rate * 3) *  # Burning increases scarcity
            (1 + daily_tasks / self.params.daily_tasks) /  # Usage increases demand
            (1 + (self.circulating / self.params.total_supply))  # Circulating increases supply
        )
        price_index = price_factors * 100
        
        # Record history
        self.history['day'].append(self.day)
        self.history['circulating'].append(self.circulating)
        self.history['staked'].append(self.staked)
        self.history['burned'].append(self.burned)
        self.history['treasury'].append(self.treasury)
        self.history['daily_revenue'].append(daily_revenue)
        self.history['burn_rate_actual'].append(actual_burn_rate)
        self.history['staking_ratio'].append(staking_ratio)
        self.history['price_index'].append(price_index)
        
        self.day += 1
        
        return {
            'day': self.day,
            'daily_tasks': daily_tasks,
            'daily_revenue': daily_revenue,
            'circulating': self.circulating,
            'staked': self.staked,
            'burned': self.burned,
            'treasury': self.treasury
        }
    
    def run_simulation(self, days: int = None):
        """Run full simulation"""
        if days is None:
            days = self.params.days
        
        print(f"Running simulation for {days} days...")
        for _ in range(days):
            self.simulate_day()
        
        print(f"✓ Simulation complete!")
        self._print_summary()
    
    def _print_summary(self):
        """Print simulation summary"""
        print("\n" + "="*60)
        print("SIMULATION SUMMARY")
        print("="*60)
        
        print(f"\nInitial State:")
        print(f"  Total Supply: {self.params.total_supply:,.0f} AINU")
        print(f"  Initial Circulating: {self.params.total_supply * self.params.initial_circulating:,.0f} AINU")
        print(f"  Initial Staked: {self.params.total_supply * self.params.initial_staked_pct:,.0f} AINU")
        
        print(f"\nFinal State (Day {self.day}):")
        print(f"  Circulating: {self.circulating:,.0f} AINU ({self.circulating/self.params.total_supply*100:.2f}%)")
        print(f"  Staked: {self.staked:,.0f} AINU ({self.staked/self.params.total_supply*100:.2f}%)")
        print(f"  Burned: {self.burned:,.0f} AINU ({self.burned/self.params.total_supply*100:.2f}%)")
        print(f"  Treasury: {self.treasury:,.0f} AINU ({self.treasury/self.params.total_supply*100:.2f}%)")
        
        total_accounted = self.circulating + self.staked + self.burned + self.treasury
        print(f"  Total Accounted: {total_accounted:,.0f} AINU ({total_accounted/self.params.total_supply*100:.2f}%)")
        
        print(f"\nEconomic Metrics:")
        print(f"  Final Daily Tasks: {self.history['daily_revenue'][-1]/self.params.avg_task_fee:,.0f}")
        print(f"  Final Daily Revenue: {self.history['daily_revenue'][-1]:,.0f} AINU")
        print(f"  Total Revenue (2 years): {sum(self.history['daily_revenue']):,.0f} AINU")
        print(f"  Staking Ratio: {self.history['staking_ratio'][-1]*100:.2f}%")
        print(f"  Burn Rate: {self.history['burn_rate_actual'][-1]*100:.2f}%")
        print(f"  Price Index: {self.history['price_index'][-1]:.2f} (100 = baseline)")
        
        # Deflationary impact
        yearly_burn = self.burned / (self.day / 365)
        print(f"\n  Yearly Burn Rate: {yearly_burn:,.0f} AINU/year ({yearly_burn/self.params.total_supply*100:.2f}%)")
        years_to_half = np.log(2) / np.log(1 + yearly_burn / self.params.total_supply)
        print(f"  Years to Half Supply: {years_to_half:.1f} years")


def plot_simulation(economy: TokenEconomy, save_path: str = None):
    """Plot simulation results"""
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    fig.suptitle('AINU Token Economic Simulation', fontsize=16, fontweight='bold')
    
    days = economy.history['day']
    
    # Plot 1: Token Distribution
    ax1 = axes[0, 0]
    ax1.plot(days, np.array(economy.history['circulating']) / 1e9, label='Circulating', linewidth=2)
    ax1.plot(days, np.array(economy.history['staked']) / 1e9, label='Staked', linewidth=2)
    ax1.plot(days, np.array(economy.history['burned']) / 1e9, label='Burned', linewidth=2)
    ax1.plot(days, np.array(economy.history['treasury']) / 1e9, label='Treasury', linewidth=2)
    ax1.set_xlabel('Days')
    ax1.set_ylabel('Tokens (Billions)')
    ax1.set_title('Token Distribution Over Time')
    ax1.legend()
    ax1.grid(True, alpha=0.3)
    
    # Plot 2: Daily Revenue
    ax2 = axes[0, 1]
    ax2.plot(days, np.array(economy.history['daily_revenue']) / 1000, linewidth=2, color='green')
    ax2.set_xlabel('Days')
    ax2.set_ylabel('Daily Revenue (Thousands AINU)')
    ax2.set_title('Daily Revenue Growth')
    ax2.grid(True, alpha=0.3)
    
    # Plot 3: Economic Metrics
    ax3 = axes[1, 0]
    ax3.plot(days, np.array(economy.history['staking_ratio']) * 100, label='Staking Ratio', linewidth=2)
    ax3.plot(days, np.array(economy.history['burn_rate_actual']) * 100, label='Cumulative Burn', linewidth=2)
    ax3.set_xlabel('Days')
    ax3.set_ylabel('Percentage (%)')
    ax3.set_title('Staking & Burn Metrics')
    ax3.legend()
    ax3.grid(True, alpha=0.3)
    
    # Plot 4: Price Index
    ax4 = axes[1, 1]
    ax4.plot(days, economy.history['price_index'], linewidth=2, color='purple')
    ax4.axhline(y=100, color='r', linestyle='--', alpha=0.5, label='Baseline')
    ax4.set_xlabel('Days')
    ax4.set_ylabel('Price Index')
    ax4.set_title('Token Price Index (Relative)')
    ax4.legend()
    ax4.grid(True, alpha=0.3)
    
    plt.tight_layout()
    
    if save_path:
        plt.savefig(save_path, dpi=150, bbox_inches='tight')
        print(f"✓ Chart saved to {save_path}")
    
    plt.show()


def run_scenarios():
    """Run multiple scenarios"""
    print("\n" + "="*60)
    print("RUNNING MULTIPLE SCENARIOS")
    print("="*60)
    
    scenarios = {
        'Base Case': SimulationParams(),
        'High Growth': SimulationParams(task_growth_rate=0.10, daily_tasks=2000),
        'Low Growth': SimulationParams(task_growth_rate=0.02, daily_tasks=500),
        'No Burn': SimulationParams(burn_enabled=False),
        'High Staking': SimulationParams(initial_staked_pct=0.40),
    }
    
    results = {}
    
    for name, params in scenarios.items():
        print(f"\n--- Scenario: {name} ---")
        economy = TokenEconomy(params)
        economy.run_simulation()
        results[name] = economy
    
    # Compare scenarios
    print("\n" + "="*60)
    print("SCENARIO COMPARISON (After 2 Years)")
    print("="*60)
    print(f"{'Scenario':<15} {'Burned':<12} {'Staked':<12} {'Price Index':<12}")
    print("-" * 60)
    
    for name, economy in results.items():
        burned_pct = economy.burned / economy.params.total_supply * 100
        staked_pct = economy.staked / economy.params.total_supply * 100
        price_idx = economy.history['price_index'][-1]
        print(f"{name:<15} {burned_pct:>10.2f}%  {staked_pct:>10.2f}%  {price_idx:>10.2f}")
    
    return results


if __name__ == '__main__':
    print("\n" + "="*60)
    print("AINU TOKEN ECONOMIC SIMULATOR")
    print("="*60)
    
    # Run base case
    print("\n📊 Running Base Case Simulation...")
    params = SimulationParams()
    economy = TokenEconomy(params)
    economy.run_simulation()
    
    # Plot results
    print("\n📈 Generating charts...")
    plot_simulation(economy, save_path='results_base_case.png')
    
    # Run multiple scenarios
    print("\n🔬 Running scenario analysis...")
    results = run_scenarios()
    
    # Export data
    print("\n💾 Exporting simulation data...")
    output = {
        'params': {
            'total_supply': params.total_supply,
            'burn_rate': params.burn_rate,
            'simulation_days': params.days
        },
        'final_state': {
            'circulating': economy.circulating,
            'staked': economy.staked,
            'burned': economy.burned,
            'treasury': economy.treasury
        },
        'history': economy.history
    }
    
    with open('simulation_results.json', 'w') as f:
        json.dump(output, f, indent=2)
    
    print("✓ Data exported to simulation_results.json")
    print("\n✅ Simulation complete!")
