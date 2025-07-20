#!/usr/bin/env python3

import json
import os
import glob
from pathlib import Path

def main():
    # Define paths
    core_data_dir = "core-data"
    output_file = "fixtures/markets.json"
    
    # Ensure fixtures directory exists
    os.makedirs("fixtures", exist_ok=True)
    
    # Find all market files
    market_files = glob.glob(os.path.join(core_data_dir, "*-markets.json"))
    
    all_markets = []
    
    for market_file in sorted(market_files):
        # Extract league name from filename (e.g., "ENG1-markets.json" -> "ENG1")
        filename = os.path.basename(market_file)
        league = filename.split("-markets.json")[0]
        
        print(f"Processing {filename} (league: {league})")
        
        try:
            # Load the market file
            with open(market_file, 'r') as f:
                markets = json.load(f)
            
            # Add league field to each market
            for market in markets:
                market["league"] = league
                all_markets.append(market)
                
            print(f"  Added {len(markets)} markets for {league}")
            
        except Exception as e:
            print(f"  Error processing {market_file}: {e}")
            continue
    
    # Save concatenated results
    print(f"\nSaving {len(all_markets)} total markets to {output_file}")
    
    with open(output_file, 'w') as f:
        json.dump(all_markets, f, indent=2)
    
    print("✓ Markets concatenation complete!")
    
    # Show summary by league
    league_counts = {}
    for market in all_markets:
        league = market["league"]
        league_counts[league] = league_counts.get(league, 0) + 1
    
    print("\nSummary by league:")
    for league, count in sorted(league_counts.items()):
        print(f"  {league}: {count} markets")

if __name__ == "__main__":
    main()