#!/usr/bin/env python3
"""
Monte Carlo Simulation for AINU Token Economics
Runs thousands of simulations with random parameters to assess risk
"""

import numpy as np
import matplotlib.pyplot as plt
from economic_model import TokenEconomy, SimulationParams
from typing import List
import json


class MonteCarloSimulator:
    """Run Monte Carlo simulations with random parameters"""
    
    def __init__(self, base_params: SimulationParams, num_simulations: int = 1000):
        self.base_params = base_params
        self.num_simulations = num_simulations
        self.results = []
    
    def generate_random_params(self) -> SimulationParams:
        """Generate random parameters with uncertainty"""
        return SimulationParams(
            total_supply=self.base_params.total_supply,  # Fixed
            initial_circulating=self.base_params.initial_circulating,  # Fixed
            
            # Vary burn rate ±20%
            burn_rate=np.random.uniform(0.04, 0.06),
            burn_enabled=True,
            
            # Vary staking APR ±30% (now centered on 20%)
            staking_apr=np.random.uniform(0.14, 0.26),
            initial_staked_pct=np.random.uniform(0.15, 0.30),
            
            # Time-based multipliers (with variation)
            has_multipliers=True,
            multiplier_3mo=1.0,
            multiplier_6mo=np.random.uniform(1.4, 1.6),  # ~1.5x
            multiplier_12mo=np.random.uniform(1.9, 2.1),  # ~2.0x
            avg_multiplier=np.random.uniform(1.3, 1.7),  # Vary based on user choice
            
            # Auto-compound rate (with variation)
            auto_compound_rate=np.random.uniform(0.40, 0.60),  # 40-60%
            
            # Vary daily tasks ±50%
            daily_tasks=int(np.random.uniform(500, 1500)),
            avg_task_fee=np.random.uniform(8, 12),
            
            # Revenue splits (fixed)
            agent_share=self.base_params.agent_share,
            node_share=self.base_params.node_share,
            protocol_share=self.base_params.protocol_share,
            burn_share=self.base_params.burn_share,
            
            # Vary growth rate ±100%
            task_growth_rate=np.random.uniform(0.02, 0.10),
            
            days=self.base_params.days
        )
    
    def run_simulations(self):
        """Run all Monte Carlo simulations"""
        print(f"\n🎲 Running {self.num_simulations} Monte Carlo simulations...")
        
        for i in range(self.num_simulations):
            if (i + 1) % 100 == 0:
                print(f"  Progress: {i+1}/{self.num_simulations} ({(i+1)/self.num_simulations*100:.0f}%)")
            
            # Generate random parameters
            params = self.generate_random_params()
            
            # Run simulation
            economy = TokenEconomy(params)
            economy.run_simulation(days=self.base_params.days)
            
            # Store results
            self.results.append({
                'params': {
                    'burn_rate': params.burn_rate,
                    'staking_apr': params.staking_apr,
                    'initial_staked_pct': params.initial_staked_pct,
                    'daily_tasks': params.daily_tasks,
                    'avg_task_fee': params.avg_task_fee,
                    'task_growth_rate': params.task_growth_rate
                },
                'final': {
                    'circulating': economy.circulating,
                    'staked': economy.staked,
                    'burned': economy.burned,
                    'treasury': economy.treasury,
                    'price_index': economy.history['price_index'][-1],
                    'staking_ratio': economy.history['staking_ratio'][-1],
                    'burn_rate_actual': economy.history['burn_rate_actual'][-1]
                }
            })
        
        print("✓ All simulations complete!")
        self._analyze_results()
    
    def _analyze_results(self):
        """Analyze Monte Carlo results"""
        print("\n" + "="*60)
        print("MONTE CARLO ANALYSIS")
        print("="*60)
        
        # Extract final states
        burned = [r['final']['burned'] for r in self.results]
        staked = [r['final']['staked'] for r in self.results]
        price_index = [r['final']['price_index'] for r in self.results]
        burn_rate = [r['final']['burn_rate_actual'] * 100 for r in self.results]
        staking_ratio = [r['final']['staking_ratio'] * 100 for r in self.results]
        
        # Calculate statistics
        print(f"\nBurned Tokens (% of supply):")
        print(f"  Mean: {np.mean(burn_rate):.2f}%")
        print(f"  Median: {np.median(burn_rate):.2f}%")
        print(f"  Std Dev: {np.std(burn_rate):.2f}%")
        print(f"  5th-95th Percentile: {np.percentile(burn_rate, 5):.2f}% - {np.percentile(burn_rate, 95):.2f}%")
        
        print(f"\nStaking Ratio:")
        print(f"  Mean: {np.mean(staking_ratio):.2f}%")
        print(f"  Median: {np.median(staking_ratio):.2f}%")
        print(f"  Std Dev: {np.std(staking_ratio):.2f}%")
        print(f"  5th-95th Percentile: {np.percentile(staking_ratio, 5):.2f}% - {np.percentile(staking_ratio, 95):.2f}%")
        
        print(f"\nPrice Index:")
        print(f"  Mean: {np.mean(price_index):.2f}")
        print(f"  Median: {np.median(price_index):.2f}")
        print(f"  Std Dev: {np.std(price_index):.2f}")
        print(f"  5th-95th Percentile: {np.percentile(price_index, 5):.2f} - {np.percentile(price_index, 95):.2f}")
        
        # Risk assessment
        print(f"\n🎯 Risk Assessment:")
        
        # Check for concerning scenarios
        low_price_scenarios = sum(1 for p in price_index if p < 80)
        high_burn_scenarios = sum(1 for b in burn_rate if b > 15)
        low_staking_scenarios = sum(1 for s in staking_ratio if s < 15)
        
        print(f"  Low Price Scenarios (<80): {low_price_scenarios} ({low_price_scenarios/self.num_simulations*100:.1f}%)")
        print(f"  High Burn Scenarios (>15%): {high_burn_scenarios} ({high_burn_scenarios/self.num_simulations*100:.1f}%)")
        print(f"  Low Staking Scenarios (<15%): {low_staking_scenarios} ({low_staking_scenarios/self.num_simulations*100:.1f}%)")
        
        # Overall risk score (lower is better)
        risk_score = (low_price_scenarios + high_burn_scenarios + low_staking_scenarios) / (self.num_simulations * 3) * 100
        print(f"\n  Overall Risk Score: {risk_score:.1f}% (lower is better)")
        
        if risk_score < 10:
            print("  ✅ Low Risk - Tokenomics appear robust")
        elif risk_score < 25:
            print("  ⚠️ Medium Risk - Some scenarios concerning, monitor closely")
        else:
            print("  ❌ High Risk - Consider adjusting parameters")
    
    def plot_distributions(self, save_path: str = None):
        """Plot Monte Carlo distributions"""
        fig, axes = plt.subplots(2, 3, figsize=(18, 10))
        fig.suptitle(f'Monte Carlo Simulation Results ({self.num_simulations} runs)', fontsize=16, fontweight='bold')
        
        # Extract data
        burn_rate = [r['final']['burn_rate_actual'] * 100 for r in self.results]
        staking_ratio = [r['final']['staking_ratio'] * 100 for r in self.results]
        price_index = [r['final']['price_index'] for r in self.results]
        daily_tasks = [r['params']['daily_tasks'] for r in self.results]
        task_growth = [r['params']['task_growth_rate'] * 100 for r in self.results]
        burned_tokens = [r['final']['burned'] / 1e9 for r in self.results]
        
        # Plot 1: Burn Rate Distribution
        axes[0, 0].hist(burn_rate, bins=50, color='red', alpha=0.7, edgecolor='black')
        axes[0, 0].axvline(np.mean(burn_rate), color='blue', linestyle='--', linewidth=2, label=f'Mean: {np.mean(burn_rate):.2f}%')
        axes[0, 0].set_xlabel('Burn Rate (%)')
        axes[0, 0].set_ylabel('Frequency')
        axes[0, 0].set_title('Cumulative Burn Rate Distribution')
        axes[0, 0].legend()
        axes[0, 0].grid(True, alpha=0.3)
        
        # Plot 2: Staking Ratio Distribution
        axes[0, 1].hist(staking_ratio, bins=50, color='green', alpha=0.7, edgecolor='black')
        axes[0, 1].axvline(np.mean(staking_ratio), color='blue', linestyle='--', linewidth=2, label=f'Mean: {np.mean(staking_ratio):.2f}%')
        axes[0, 1].set_xlabel('Staking Ratio (%)')
        axes[0, 1].set_ylabel('Frequency')
        axes[0, 1].set_title('Final Staking Ratio Distribution')
        axes[0, 1].legend()
        axes[0, 1].grid(True, alpha=0.3)
        
        # Plot 3: Price Index Distribution
        axes[0, 2].hist(price_index, bins=50, color='purple', alpha=0.7, edgecolor='black')
        axes[0, 2].axvline(np.mean(price_index), color='blue', linestyle='--', linewidth=2, label=f'Mean: {np.mean(price_index):.2f}')
        axes[0, 2].axvline(100, color='red', linestyle='--', linewidth=2, label='Baseline: 100')
        axes[0, 2].set_xlabel('Price Index')
        axes[0, 2].set_ylabel('Frequency')
        axes[0, 2].set_title('Final Price Index Distribution')
        axes[0, 2].legend()
        axes[0, 2].grid(True, alpha=0.3)
        
        # Plot 4: Burned Tokens
        axes[1, 0].hist(burned_tokens, bins=50, color='orange', alpha=0.7, edgecolor='black')
        axes[1, 0].axvline(np.mean(burned_tokens), color='blue', linestyle='--', linewidth=2, label=f'Mean: {np.mean(burned_tokens):.2f}B')
        axes[1, 0].set_xlabel('Burned Tokens (Billions)')
        axes[1, 0].set_ylabel('Frequency')
        axes[1, 0].set_title('Total Burned Tokens Distribution')
        axes[1, 0].legend()
        axes[1, 0].grid(True, alpha=0.3)
        
        # Plot 5: Price vs Growth Scatter
        axes[1, 1].scatter(task_growth, price_index, alpha=0.5, s=20)
        axes[1, 1].set_xlabel('Task Growth Rate (%/month)')
        axes[1, 1].set_ylabel('Price Index')
        axes[1, 1].set_title('Price vs Growth Rate Correlation')
        axes[1, 1].grid(True, alpha=0.3)
        
        # Plot 6: Price vs Staking Scatter
        axes[1, 2].scatter(staking_ratio, price_index, alpha=0.5, s=20, color='green')
        axes[1, 2].set_xlabel('Staking Ratio (%)')
        axes[1, 2].set_ylabel('Price Index')
        axes[1, 2].set_title('Price vs Staking Correlation')
        axes[1, 2].grid(True, alpha=0.3)
        
        plt.tight_layout()
        
        if save_path:
            plt.savefig(save_path, dpi=150, bbox_inches='tight')
            print(f"✓ Monte Carlo chart saved to {save_path}")
        
        plt.show()
    
    def export_results(self, filepath: str):
        """Export Monte Carlo results to JSON"""
        output = {
            'num_simulations': self.num_simulations,
            'base_params': {
                'total_supply': self.base_params.total_supply,
                'burn_rate': self.base_params.burn_rate,
                'simulation_days': self.base_params.days
            },
            'statistics': {
                'burn_rate': {
                    'mean': float(np.mean([r['final']['burn_rate_actual'] * 100 for r in self.results])),
                    'median': float(np.median([r['final']['burn_rate_actual'] * 100 for r in self.results])),
                    'std': float(np.std([r['final']['burn_rate_actual'] * 100 for r in self.results])),
                    'p5': float(np.percentile([r['final']['burn_rate_actual'] * 100 for r in self.results], 5)),
                    'p95': float(np.percentile([r['final']['burn_rate_actual'] * 100 for r in self.results], 95))
                },
                'staking_ratio': {
                    'mean': float(np.mean([r['final']['staking_ratio'] * 100 for r in self.results])),
                    'median': float(np.median([r['final']['staking_ratio'] * 100 for r in self.results])),
                    'std': float(np.std([r['final']['staking_ratio'] * 100 for r in self.results])),
                    'p5': float(np.percentile([r['final']['staking_ratio'] * 100 for r in self.results], 5)),
                    'p95': float(np.percentile([r['final']['staking_ratio'] * 100 for r in self.results], 95))
                },
                'price_index': {
                    'mean': float(np.mean([r['final']['price_index'] for r in self.results])),
                    'median': float(np.median([r['final']['price_index'] for r in self.results])),
                    'std': float(np.std([r['final']['price_index'] for r in self.results])),
                    'p5': float(np.percentile([r['final']['price_index'] for r in self.results], 5)),
                    'p95': float(np.percentile([r['final']['price_index'] for r in self.results], 95))
                }
            }
        }
        
        with open(filepath, 'w') as f:
            json.dump(output, f, indent=2)
        
        print(f"✓ Monte Carlo results exported to {filepath}")


if __name__ == '__main__':
    print("\n" + "="*60)
    print("MONTE CARLO SIMULATION FOR AINU TOKEN")
    print("="*60)
    
    # Run Monte Carlo with 1000 simulations
    base_params = SimulationParams()
    simulator = MonteCarloSimulator(base_params, num_simulations=1000)
    simulator.run_simulations()
    
    # Plot distributions
    print("\n📊 Generating Monte Carlo distributions...")
    simulator.plot_distributions(save_path='monte_carlo_results.png')
    
    # Export results
    print("\n💾 Exporting Monte Carlo results...")
    simulator.export_results('monte_carlo_results.json')
    
    print("\n✅ Monte Carlo analysis complete!")
