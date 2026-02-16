package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bryan-buckman/infovore/internal/kalshi/models"
)

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Kalshi Markets Report</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --primary: #2563eb;
            --primary-dark: #1d4ed8;
            --success: #16a34a;
            --danger: #dc2626;
            --warning: #d97706;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 2rem;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        header {
            text-align: center;
            margin-bottom: 2rem;
            padding-bottom: 1.5rem;
            border-bottom: 2px solid var(--border);
        }
        h1 {
            font-size: 2rem;
            color: var(--primary);
            margin-bottom: 0.5rem;
        }
        .subtitle {
            color: var(--text-muted);
            font-size: 1rem;
        }
        .stats {
            display: flex;
            justify-content: center;
            gap: 2rem;
            margin: 1.5rem 0;
            flex-wrap: wrap;
        }
        .stat {
            background: var(--card-bg);
            padding: 1rem 2rem;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            text-align: center;
        }
        .stat-value {
            font-size: 2rem;
            font-weight: bold;
            color: var(--primary);
        }
        .stat-label {
            color: var(--text-muted);
            font-size: 0.875rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .filters {
            background: var(--card-bg);
            padding: 1rem;
            border-radius: 8px;
            margin-bottom: 1.5rem;
            text-align: center;
            color: var(--text-muted);
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: var(--card-bg);
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        th {
            background: var(--primary);
            color: white;
            padding: 1rem;
            text-align: left;
            font-weight: 600;
            font-size: 0.875rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            cursor: pointer;
            user-select: none;
            position: relative;
            transition: background-color 0.2s;
        }
        th:hover {
            background: var(--primary-dark);
        }
        th::after {
            content: '';
            position: absolute;
            right: 0.5rem;
            opacity: 0.5;
        }
        th.sort-asc::after {
            content: ' ▲';
            opacity: 1;
        }
        th.sort-desc::after {
            content: ' ▼';
            opacity: 1;
        }
        td {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            vertical-align: top;
        }
        tr:hover {
            background: #f1f5f9;
        }
        tr:last-child td {
            border-bottom: none;
        }
        .market-title {
            font-weight: 600;
            color: var(--text);
            margin-bottom: 0.25rem;
        }
        .market-subtitle {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .price {
            font-weight: 600;
            font-size: 1.125rem;
        }
        .price-ask {
            color: var(--success);
        }
        .price-bid {
            color: var(--text-muted);
        }
        .spread {
            font-size: 0.875rem;
            color: var(--warning);
        }
        .side {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 4px;
            font-weight: 600;
            font-size: 0.875rem;
        }
        .side-yes {
            background: #dcfce7;
            color: var(--success);
        }
        .side-no {
            background: #fee2e2;
            color: var(--danger);
        }
        .ticker {
            font-family: 'SF Mono', Monaco, 'Courier New', monospace;
            font-size: 0.75rem;
            background: #f1f5f9;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            color: var(--text-muted);
        }
        .series {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .expiration {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .expiration-soon {
            color: var(--warning);
            font-weight: 600;
        }
        .implied-prob {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        footer {
            text-align: center;
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
            color: var(--text-muted);
            font-size: 0.875rem;
        }
        /* Portfolio section */
        .portfolio-section {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .portfolio-section h2 {
            color: var(--primary);
            font-size: 1.25rem;
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .portfolio-stats {
            display: flex;
            gap: 1.5rem;
            margin-bottom: 1rem;
            flex-wrap: wrap;
        }
        .portfolio-stat {
            background: var(--bg);
            padding: 0.75rem 1.25rem;
            border-radius: 8px;
            text-align: center;
            min-width: 120px;
        }
        .portfolio-stat-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary);
        }
        .portfolio-stat-label {
            font-size: 0.75rem;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .positions-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }
        .positions-table th {
            background: #eef2ff;
            color: var(--primary);
            padding: 0.5rem 0.75rem;
            text-align: left;
            font-size: 0.75rem;
            cursor: default;
        }
        .positions-table th:hover {
            background: #e0e7ff;
        }
        .positions-table td {
            padding: 0.5rem 0.75rem;
            border-bottom: 1px solid var(--border);
        }
        .no-positions {
            color: var(--text-muted);
            font-style: italic;
            padding: 1rem 0;
        }
        /* Portfolio edge input */
        .portfolio-edge-input {
            width: 4.5rem;
            background: #f8fafc;
            border: 1px solid #cbd5e1;
            border-radius: 4px;
            padding: 3px 6px;
            text-align: right;
            font-size: 0.85rem;
            color: var(--text);
        }
        .portfolio-edge-input:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.2);
        }
        .theoretical-row {
            background: #eff6ff !important;
        }
        .theoretical-row:hover {
            background: #dbeafe !important;
        }
        .new-badge {
            display: inline-block;
            background: #dbeafe;
            color: var(--primary);
            font-size: 0.65rem;
            font-weight: 600;
            padding: 1px 6px;
            border-radius: 3px;
            margin-left: 0.5rem;
            vertical-align: middle;
        }
        #portfolio-table th {
            cursor: pointer;
        }
        /* Checkbox column */
        .select-col {
            width: 40px;
            text-align: center;
        }
        .select-col input[type="checkbox"] {
            width: 18px;
            height: 18px;
            cursor: pointer;
            accent-color: var(--primary);
        }
        /* Kelly panel */
        .kelly-panel {
            background: linear-gradient(135deg, #1e293b, #334155);
            color: white;
            border-radius: 12px;
            padding: 1.5rem;
            margin-top: 1.5rem;
            display: none;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        }
        .kelly-panel.visible {
            display: block;
            animation: slideIn 0.3s ease-out;
        }
        @keyframes slideIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }
        .kelly-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
            flex-wrap: wrap;
            gap: 1rem;
        }
        .kelly-header h2 {
            color: #93c5fd;
            font-size: 1.25rem;
            margin: 0;
        }
        .bankroll-input {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .bankroll-input label {
            color: #94a3b8;
            font-size: 0.875rem;
        }
        .bankroll-input input {
            background: #475569;
            border: 1px solid #64748b;
            color: white;
            padding: 0.5rem 0.75rem;
            border-radius: 6px;
            font-size: 0.875rem;
            width: 120px;
            text-align: right;
        }
        .bankroll-input input:focus {
            outline: none;
            border-color: #93c5fd;
            box-shadow: 0 0 0 2px rgba(147, 197, 253, 0.3);
        }
        .kelly-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 0.5rem;
        }
        .kelly-table th {
            background: rgba(255,255,255,0.1);
            color: #93c5fd;
            padding: 0.75rem;
            text-align: left;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            cursor: default;
        }
        .kelly-table th:hover {
            background: rgba(255,255,255,0.15);
        }
        .kelly-table td {
            padding: 0.75rem;
            border-bottom: 1px solid rgba(255,255,255,0.1);
            font-size: 0.875rem;
        }
        .kelly-table tr:last-child td {
            border-bottom: none;
        }
        .kelly-positive {
            color: #86efac;
            font-weight: 600;
        }
        .pnl-negative {
            color: #fca5a5;
            font-weight: 600;
        }
        .kelly-zero {
            color: #94a3b8;
        }
        .kelly-summary {
            margin-top: 1rem;
            padding-top: 1rem;
            border-top: 1px solid rgba(255,255,255,0.1);
            display: flex;
            gap: 2rem;
            flex-wrap: wrap;
        }
        .kelly-summary-item {
            text-align: center;
        }
        .kelly-summary-value {
            font-size: 1.25rem;
            font-weight: 700;
            color: #93c5fd;
        }
        .kelly-summary-label {
            font-size: 0.75rem;
            color: #94a3b8;
            text-transform: uppercase;
        }
        /* Hamburger menu */
        .hamburger-btn {
            position: fixed;
            top: 1rem;
            left: 1rem;
            z-index: 1000;
            width: 44px;
            height: 44px;
            border: none;
            background: var(--primary);
            color: white;
            font-size: 1.5rem;
            border-radius: 8px;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 2px 8px rgba(0,0,0,0.15);
            transition: background-color 0.2s;
        }
        .hamburger-btn:hover {
            background: var(--primary-dark);
        }
        .menu-dropdown {
            display: none;
            position: fixed;
            top: 4rem;
            left: 1rem;
            z-index: 1000;
            background: var(--card-bg);
            border-radius: 8px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.15);
            min-width: 200px;
            overflow: hidden;
        }
        .menu-dropdown.visible {
            display: block;
        }
        .menu-dropdown button {
            display: block;
            width: 100%;
            padding: 0.75rem 1rem;
            border: none;
            background: none;
            text-align: left;
            font-size: 0.95rem;
            cursor: pointer;
            color: var(--text);
        }
        .menu-dropdown button:hover {
            background: var(--bg);
        }
        /* Settings dialog */
        .settings-overlay {
            display: none;
            position: fixed;
            inset: 0;
            z-index: 2000;
            background: rgba(0,0,0,0.4);
            align-items: center;
            justify-content: center;
        }
        .settings-overlay.visible {
            display: flex;
        }
        .settings-dialog {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 2rem;
            max-width: 500px;
            width: 90%;
            max-height: 80vh;
            overflow-y: auto;
            box-shadow: 0 8px 30px rgba(0,0,0,0.2);
        }
        .settings-dialog h3 {
            font-size: 1.25rem;
            margin-bottom: 1rem;
            color: var(--primary);
        }
        .settings-dialog h4 {
            font-size: 0.9rem;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 0.75rem;
        }
        .category-list {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 0.5rem;
            margin-bottom: 1.5rem;
        }
        .category-checkbox {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.875rem;
            transition: background-color 0.15s;
        }
        .category-checkbox:hover {
            background: var(--bg);
        }
        .category-checkbox input[type="checkbox"] {
            width: 18px;
            height: 18px;
            accent-color: var(--primary);
        }
        .category-badge {
            display: inline-block;
            padding: 0.15rem 0.5rem;
            border-radius: 3px;
            font-size: 0.7rem;
            font-weight: 600;
            background: #eef2ff;
            color: var(--primary);
            margin-left: 0.25rem;
        }
        .settings-actions {
            display: flex;
            gap: 0.75rem;
            justify-content: flex-end;
        }
        .settings-actions button {
            padding: 0.5rem 1.25rem;
            border-radius: 6px;
            font-size: 0.875rem;
            cursor: pointer;
            border: 1px solid var(--border);
            background: var(--bg);
            color: var(--text);
        }
        .settings-actions button:first-child {
            background: var(--primary);
            color: white;
            border-color: var(--primary);
        }
        .settings-actions button:first-child:hover {
            background: var(--primary-dark);
        }
        .settings-note {
            color: var(--text-muted);
            font-size: 0.75rem;
            margin-top: 0.75rem;
            text-align: center;
        }
        .settings-status {
            color: var(--success);
            font-size: 0.8rem;
            margin-top: 0.5rem;
            text-align: center;
            display: none;
        }
        @media (max-width: 768px) {
            body {
                padding: 1rem;
            }
            .stats {
                flex-direction: column;
                align-items: center;
            }
            table {
                font-size: 0.875rem;
            }
            th, td {
                padding: 0.75rem 0.5rem;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <!-- Hamburger menu -->
        <button class="hamburger-btn" id="hamburger-btn" title="Menu">&#9776;</button>
        <div class="menu-dropdown" id="menu-dropdown">
            <button id="open-settings">&#9881; Settings</button>
        </div>

        <!-- Settings dialog -->
        <div class="settings-overlay" id="settings-overlay">
            <div class="settings-dialog">
                <h3>&#9881; Settings</h3>
                <h4>Market Categories</h4>
                <p style="color:var(--text-muted);font-size:0.8rem;margin-bottom:0.75rem;">Select which categories to include in the scan. Changes take effect when you click Save &amp; Refresh.</p>
                <div class="category-list" id="category-list">
                    {{range .AllCategories}}
                    <label class="category-checkbox">
                        <input type="checkbox" value="{{.Name}}" {{if .Active}}checked{{end}}>
                        {{.Name}}
                    </label>
                    {{end}}
                </div>
                <div class="settings-actions">
                    <button id="settings-save">Save &amp; Refresh</button>
                    <button id="settings-cancel">Cancel</button>
                </div>
                <p class="settings-note">Categories are saved to the server. A new scan will start with the selected categories.</p>
                <p class="settings-status" id="settings-status">Settings saved! Refreshing data...</p>
            </div>
        </div>

        <header>
            <h1>Kalshi Markets Report</h1>
            <p style="color:var(--text-muted);font-size:0.85rem;margin-top:0.25rem;">Categories: {{.ActiveCategories}}</p>
            <p class="subtitle">Generated on {{.GeneratedAt}}</p>
        </header>

        <div class="stats">
            <div class="stat">
                <div class="stat-value">{{.TotalContracts}}</div>
                <div class="stat-label">Contracts Found</div>
            </div>
            <div class="stat">
                <div class="stat-value">{{.TotalSeries}}</div>
                <div class="stat-label">Series Scanned</div>
            </div>
            <div class="stat">
                <div class="stat-value">{{printf "%.0f" .MinPrice}}¢ - {{printf "%.0f" .MaxPrice}}¢</div>
                <div class="stat-label">Price Range</div>
            </div>
        </div>

        <div class="portfolio-section" id="portfolio-section">
            <h2>📊 Portfolio</h2>
            {{if .HasPortfolio}}
            <div class="portfolio-stats">
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">${{printf "%.2f" .CashBalance}}</div>
                    <div class="portfolio-stat-label">Cash Balance</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">${{printf "%.2f" .PortfolioValue}}</div>
                    <div class="portfolio-stat-label">Portfolio Value</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">${{printf "%.2f" .TotalBankroll}}</div>
                    <div class="portfolio-stat-label">Total Bankroll</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">{{.NumPositions}}</div>
                    <div class="portfolio-stat-label">Open Positions</div>
                </div>
            </div>
            {{end}}
            <p style="color:var(--text-muted);font-size:0.75rem;margin-bottom:0.5rem;">Edge = raw favorite-longshot bias estimate (editable). Kalshi taker fee is auto-deducted before Kelly calculation. Check markets below to add theoretical positions.</p>
            <table class="positions-table" id="portfolio-table">
                <thead>
                    <tr>
                        <th>Market</th>
                        <th>Side</th>
                        <th>Qty</th>
                        <th>Purchase Price</th>
                        <th>Ask</th>
                        <th>Bid</th>
                        <th>Exposure</th>
                        <th>Unrealized P&L</th>
                        <th>Expires</th>
                        <th title="Raw bias edge estimate. Kalshi taker fee is auto-deducted before Kelly calculation.">Edge (¢)</th>
                        <th>Full Kelly</th>
                        <th>½ Kelly</th>
                        <th>¼ Kelly</th>
                    </tr>
                </thead>
                <tbody id="portfolio-body">
                    {{range .PortfolioPositions}}
                    <tr class="portfolio-row" data-ask="{{printf "%.4f" .AskPrice}}" data-bid="{{printf "%.4f" .BidPrice}}" data-ticker="{{.Ticker}}" data-side="{{.Side}}" data-series="{{.SeriesTicker}}">
                        <td>
                            <div class="market-title">{{.Title}}</div>
                            <div class="market-subtitle">{{.Ticker}}</div>
                        </td>
                        <td><span class="side {{if eq .Side "YES"}}side-yes{{else}}side-no{{end}}">{{.Side}}</span></td>
                        <td>{{.Contracts}}</td>
                        <td>{{.PurchasePrice}}</td>
                        <td>{{.CurrentAskCents}}</td>
                        <td>{{.CurrentBidCents}}</td>
                        <td>{{.Exposure}}</td>
                        <td>{{.UnrealizedPnL}}</td>
                        <td data-value="{{.ExpirationUnix}}">{{.ExpirationStr}}</td>
                        <td><input type="number" class="portfolio-edge-input" value="{{printf "%.1f" .RawEdgeCents}}" step="0.1" title="Fee: {{printf "%.1f" .FeeCents}}¢ auto-deducted">¢</td>
                        <td class="pf-kelly-full kelly-positive">{{.KellyStr}}</td>
                        <td class="pf-kelly-half">{{.HalfKellyStr}}</td>
                        <td class="pf-kelly-quarter">{{.QuarterKellyStr}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        {{if .Transactions}}
        <div class="portfolio-section" id="transactions-section">
            <h2>💰 Transactions <span style="font-size:0.75rem;color:var(--text-muted);font-weight:400;">{{.TransactionYear}} YTD</span></h2>
            <div class="portfolio-stats">
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">{{.TransactionCount}}</div>
                    <div class="portfolio-stat-label">Transactions</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value {{if ge .TotalGainLoss 0.0}}kelly-positive{{else}}pnl-negative{{end}}">${{printf "%.2f" .TotalGainLoss}}</div>
                    <div class="portfolio-stat-label">Total Gain/Loss</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">${{printf "%.2f" .TotalFees}}</div>
                    <div class="portfolio-stat-label">Total Fees</div>
                </div>
                <div class="portfolio-stat">
                    <div class="portfolio-stat-value">${{printf "%.2f" .TotalTax}}</div>
                    <div class="portfolio-stat-label">Est. Tax (21%)</div>
                </div>
            </div>
            <table class="positions-table" id="transactions-table">
                <thead>
                    <tr>
                        <th>Market</th>
                        <th>Type</th>
                        <th>Side</th>
                        <th>Qty</th>
                        <th>Date</th>
                        <th>Purchase Price</th>
                        <th>Settlement Price</th>
                        <th>Gain/Loss</th>
                        <th>Fees</th>
                        <th>Tax (21%)</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Transactions}}
                    <tr>
                        <td>
                            <div class="market-title">{{.Title}}</div>
                            <div class="market-subtitle">{{.Ticker}}</div>
                        </td>
                        <td><span class="side {{if eq .Type "Settlement"}}side-yes{{else}}side-no{{end}}">{{.Type}}</span></td>
                        <td><span class="side {{if eq .Side "YES"}}side-yes{{else}}side-no{{end}}">{{.Side}}</span></td>
                        <td>{{.Contracts}}</td>
                        <td data-value="{{.DateUnix}}">{{.Date}}</td>
                        <td>{{.CostStr}}</td>
                        <td>{{.RevenueStr}}</td>
                        <td class="{{if ge .GainLoss 0.0}}kelly-positive{{else}}pnl-negative{{end}}">{{.GainLossStr}}</td>
                        <td>{{.FeesStr}}</td>
                        <td>{{.TaxStr}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}

        <div class="portfolio-section" id="markets-section">
            <h2>📈 Markets</h2>
            <div class="filters">
                Showing YES and NO contracts with ask prices between ${{printf "%.2f" .MinPriceDollars}} and ${{printf "%.2f" .MaxPriceDollars}} ({{printf "%.0f" .MinPrice}}¢ - {{printf "%.0f" .MaxPrice}}¢)
            </div>

        {{if .Markets}}
        <table id="markets-table">
            <thead>
                <tr>
                    <th class="select-col"><input type="checkbox" id="select-all" title="Select All"></th>
                    <th>Market</th>
                    <th>Side</th>
                    <th>Ask Price</th>
                    <th>Bid Price</th>
                    <th>Spread</th>
                    <th>Expires</th>
                    <th>Ticker</th>
                </tr>
            </thead>
            <tbody>
                {{range .Markets}}
                <tr data-category="{{.Market.Category}}">
                    <td class="select-col">
                        <input type="checkbox" class="market-select"
                            data-index="{{.Index}}"
                            data-ticker="{{.Market.Market.Ticker}}"
                            data-title="{{.Market.Market.Title}}"
                            data-side="{{.Side}}"
                            data-ask="{{printf "%.4f" .AskDollars}}"
                            data-bid="{{printf "%.4f" .BidDollars}}"
                            data-series="{{.Market.Market.SeriesTicker}}"
                            data-expires="{{.ExpirationStr}}">
                    </td>
                    <td data-value="{{.Market.Market.Title}}">
                        <div class="market-title">{{.Market.Market.Title}}</div>
                        {{if .Market.Market.Subtitle}}<div class="market-subtitle">{{.Market.Market.Subtitle}}</div>{{end}}
                            {{if .Market.Category}}<span class="category-badge">{{.Market.Category}}</span>{{end}}
                    </td>
                    <td data-value="{{.Side}}">
                        <span class="side {{if eq .Side "YES"}}side-yes{{else}}side-no{{end}}">{{.Side}}</span>
                    </td>
                    <td data-value="{{printf "%.2f" .AskCents}}">
                        <div class="price price-ask">{{printf "%.0f" .AskCents}}¢</div>
                        <div class="implied-prob">{{printf "%.0f" .AskCents}}% implied</div>
                    </td>
                    <td data-value="{{printf "%.2f" .BidCents}}">
                        <div class="price price-bid">{{printf "%.0f" .BidCents}}¢</div>
                    </td>
                    <td data-value="{{printf "%.2f" .SpreadCents}}">
                        <div class="spread">{{printf "%.0f" .SpreadCents}}¢</div>
                    </td>
                    <td data-value="{{.Market.Expiration.Unix}}">
                        <div class="expiration{{if .ExpiresSoon}} expiration-soon{{end}}">{{.ExpirationStr}}</div>
                    </td>
                    <td data-value="{{.Market.Market.Ticker}}">
                        <span class="ticker">{{.Market.Market.Ticker}}</span>
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
        {{else}}
        <div class="filters" style="padding: 3rem;">
            <p style="font-size: 1.25rem; color: var(--text);">No contracts found in this price range.</p>
            <p>Try adjusting the price filters or check back later.</p>
        </div>
        {{end}}
        </div>

        <!-- Existing positions JSON for JavaScript -->
        <script id="existing-positions-data" type="application/json">{{.PositionsJSON}}</script>

        <!-- Kelly Criterion Portfolio Optimizer -->
        <div class="kelly-panel" id="kelly-panel">
            <div class="kelly-header">
                <h2>🎯 Portfolio Optimizer (Kelly Criterion)</h2>
                <div class="bankroll-input">
                    <label for="bankroll">Bankroll: $</label>
                    <input type="number" id="bankroll" value="{{printf "%.2f" .TotalBankroll}}" min="0" step="0.01">
                </div>
            </div>
            <p style="color:#94a3b8;font-size:0.8rem;margin:0 0 0.75rem 0;">Edge estimated from favorite-longshot bias calibration data, adjusted for Kalshi taker fees. Edge values are editable per row. Kelly treats each position independently.</p>
            <table class="kelly-table">
                <thead>
                    <tr>
                        <th>Status</th>
                        <th>Market</th>
                        <th>Side</th>
                        <th>Ask</th>
                        <th>Bid</th>
                        <th>Edge</th>
                        <th>Full Kelly</th>
                        <th>½ Kelly</th>
                        <th>¼ Kelly</th>
                    </tr>
                </thead>
                <tbody id="kelly-body">
                </tbody>
            </table>
            <div class="kelly-summary" id="kelly-summary"></div>
        </div>

        <footer>
            <p>Data sourced from Kalshi API | Report generated by kalshi-api tool</p>
            <p>Prices shown are contract ask prices. Always verify current prices before trading.</p>
            <p style="margin-top: 0.5rem; font-size: 0.75rem;">Kelly Criterion recommendations are mathematical suggestions only, not financial advice. Past performance does not guarantee future results.</p>
        </footer>
    </div>
    <script>
    (function() {
        // ===== Table Sorting =====
        var table = document.getElementById('markets-table');
        if (table) {
        var headers = table.querySelectorAll('th');
        var tbody = table.querySelector('tbody');
        var currentSort = { column: -1, ascending: true };
        
        // Columns that should be sorted numerically (0-indexed, accounting for checkbox col)
        const numericColumns = [3, 4, 5, 6]; // Ask, Bid, Spread, Expires (unix timestamp)
        const skipColumns = [0]; // Don't sort by checkbox column
        
        headers.forEach((header, index) => {
            if (skipColumns.includes(index)) return;
            header.addEventListener('click', () => {
                const ascending = currentSort.column === index ? !currentSort.ascending : true;
                sortTable(index, ascending);
                currentSort = { column: index, ascending };
                
                headers.forEach(h => h.classList.remove('sort-asc', 'sort-desc'));
                header.classList.add(ascending ? 'sort-asc' : 'sort-desc');
            });
        });
        
        function sortTable(columnIndex, ascending) {
            const rows = Array.from(tbody.querySelectorAll('tr'));
            const isNumeric = numericColumns.includes(columnIndex);
            
            rows.sort((a, b) => {
                const aCell = a.cells[columnIndex];
                const bCell = b.cells[columnIndex];
                const aValue = aCell.dataset.value || aCell.textContent.trim();
                const bValue = bCell.dataset.value || bCell.textContent.trim();
                
                let comparison;
                if (isNumeric) {
                    comparison = parseFloat(aValue) - parseFloat(bValue);
                } else {
                    comparison = aValue.localeCompare(bValue);
                }
                
                return ascending ? comparison : -comparison;
            });
            
            rows.forEach(row => tbody.appendChild(row));
        }

        } // end table-sorting block

        // ===== State for edge editing =====
        var lastCombined = [];
        var customEdges = {};

        // ===== Kalshi Fee Calculation =====
        function kalshiFee(price) {
            if (price <= 0 || price >= 1) return 0;
            return Math.ceil(0.07 * price * (1 - price) * 100) / 100;
        }

        // ===== Kelly Criterion Calculation (Favorite-Longshot Bias) =====
        function favoriteLongshotEdge(ask) {
            if (ask <= 0 || ask >= 1) return 0;
            if (ask < 0.10) return -0.07;
            if (ask < 0.50) return -0.05 + (ask - 0.10) / (0.50 - 0.10) * (-0.02 - (-0.05));
            if (ask < 0.55) return -0.02 + (ask - 0.50) / (0.55 - 0.50) * (0.01 - (-0.02));
            if (ask < 0.85) return 0.01 + (ask - 0.55) / (0.85 - 0.55) * (0.05 - 0.01);
            return 0.04;
        }

        function calculateKelly(askPrice, bidPrice, customEdge) {
            if (askPrice <= 0 || askPrice >= 1) {
                return { kelly: 0, edge: 0, trueProb: 0, fee: 0 };
            }
            var fee = kalshiFee(askPrice);
            var biasEdge = (typeof customEdge === 'number') ? customEdge : favoriteLongshotEdge(askPrice);
            var effectiveEdge = biasEdge - fee;
            var trueProb = Math.max(0.001, Math.min(0.999, askPrice + effectiveEdge));
            var b = (1 - askPrice) / askPrice;
            var p = trueProb;
            var q = 1 - p;
            var kelly = (b * p - q) / b;
            if (kelly <= 0) return { kelly: 0, edge: effectiveEdge, trueProb: trueProb, fee: fee };
            kelly = Math.min(kelly, 1.0);
            return { kelly: kelly, edge: effectiveEdge, trueProb: trueProb, fee: fee };
        }

        function formatDollars(amount) {
            return '$' + amount.toFixed(2);
        }

        function formatPercent(fraction) {
            return (fraction * 100).toFixed(1) + '%';
        }

        function formatCents(dollars) {
            return (dollars * 100).toFixed(0) + '\u00a2';
        }

        // ===== Load existing positions =====
        var existingPositions = [];
        try {
            var posDataEl = document.getElementById('existing-positions-data');
            if (posDataEl && posDataEl.textContent.trim()) {
                existingPositions = JSON.parse(posDataEl.textContent);
            }
        } catch(e) { /* no existing positions */ }

        // ===== Checkbox & Panel Logic =====
        var kellyPanel = document.getElementById('kelly-panel');
        var kellyBody = document.getElementById('kelly-body');
        var kellySummary = document.getElementById('kelly-summary');
        var bankrollInput = document.getElementById('bankroll');

        function getSelectedMarkets() {
            var selected = [];
            document.querySelectorAll('.market-select').forEach(function(cb) {
                if (cb.checked) {
                    selected.push({
                        ticker: cb.dataset.ticker,
                        title: cb.dataset.title,
                        side: cb.dataset.side,
                        ask: parseFloat(cb.dataset.ask),
                        bid: parseFloat(cb.dataset.bid),
                        series: cb.dataset.series || '',
                        status: 'NEW'
                    });
                }
            });
            return selected;
        }

        function buildCombinedPortfolio() {
            var combined = [];
            // Add existing positions
            existingPositions.forEach(function(p) {
                var key = p.ticker + '|' + p.side;
                var ce = customEdges[key];
                var result = (typeof ce === 'number') ? calculateKelly(p.ask, p.bid, ce) : calculateKelly(p.ask, p.bid);
                combined.push({
                    ticker: p.ticker,
                    title: p.title,
                    side: p.side,
                    ask: p.ask,
                    bid: p.bid,
                    series: p.series || '',
                    edge: result.edge,
                    kelly: result.kelly,
                    halfKelly: result.kelly / 2,
                    quarterKelly: result.kelly / 4,
                    status: 'CURRENT',
                    customEdge: ce
                });
            });
            // Add selected new markets
            var selected = getSelectedMarkets();
            selected.forEach(function(m) {
                var key = m.ticker + '|' + m.side;
                var ce = customEdges[key];
                var result = (typeof ce === 'number') ? calculateKelly(m.ask, m.bid, ce) : calculateKelly(m.ask, m.bid);
                combined.push({
                    ticker: m.ticker,
                    title: m.title,
                    side: m.side,
                    ask: m.ask,
                    bid: m.bid,
                    series: m.series || '',
                    edge: result.edge,
                    kelly: result.kelly,
                    halfKelly: result.kelly / 2,
                    quarterKelly: result.kelly / 4,
                    status: 'NEW',
                    customEdge: ce
                });
            });
            return combined;
        }

        function renderRow(r, bankroll, idx) {
            var sideClass = r.side === 'YES' ? 'side-yes' : 'side-no';
            var edgeClass = r.edge > 0 ? 'kelly-positive' : 'kelly-zero';
            var statusClass = r.status === 'CURRENT' ? 'side-yes' : 'side-no';
            var kellyClass = r.kelly > 0 ? 'kelly-positive' : 'kelly-zero';
            var fullD = r.kelly * bankroll;
            var halfD = r.halfKelly * bankroll;
            var quarterD = r.quarterKelly * bankroll;
            var seriesTag = r.series ? ' <span class="ticker" style="font-size:0.65rem;">' + r.series + '</span>' : '';
            var edgeCents = (typeof r.customEdge === 'number') ? (r.customEdge * 100).toFixed(1) : (favoriteLongshotEdge(r.ask) * 100).toFixed(1);
            return '<tr data-idx="' + idx + '">' +
                '<td><span class="side ' + statusClass + '">' + r.status + '</span></td>' +
                '<td>' + r.title + seriesTag + '</td>' +
                '<td><span class="side ' + sideClass + '">' + r.side + '</span></td>' +
                '<td>' + formatCents(r.ask) + '</td>' +
                '<td>' + formatCents(r.bid) + '</td>' +
                '<td><input type="number" class="edge-input" data-idx="' + idx + '" value="' + edgeCents + '" step="0.1" style="width:4.5rem;background:rgba(255,255,255,0.08);border:1px solid rgba(255,255,255,0.15);border-radius:4px;color:inherit;padding:2px 4px;text-align:right;font-size:0.85rem;" title="Edge in cents (editable)">\u00a2</td>' +
                '<td class="' + kellyClass + '">' + formatPercent(r.kelly) + ' \u00b7 ' + formatDollars(fullD) + '</td>' +
                '<td>' + formatPercent(r.halfKelly) + ' \u00b7 ' + formatDollars(halfD) + '</td>' +
                '<td>' + formatPercent(r.quarterKelly) + ' \u00b7 ' + formatDollars(quarterD) + '</td>' +
                '</tr>';
        }

        function renderSummary(combined, bankroll) {
            var totalFull = 0, totalHalf = 0, totalQuarter = 0;
            var numCurrent = 0, numNew = 0;
            combined.forEach(function(r) {
                if (r.status === 'CURRENT') numCurrent++; else numNew++;
                if (r.kelly > 0) {
                    totalFull += r.kelly * bankroll;
                    totalHalf += r.halfKelly * bankroll;
                    totalQuarter += r.quarterKelly * bankroll;
                }
            });

            kellySummary.innerHTML =
                '<div class="kelly-summary-item">' +
                    '<div class="kelly-summary-value">' + numCurrent + ' / ' + numNew + '</div>' +
                    '<div class="kelly-summary-label">Current / New</div>' +
                '</div>' +
                '<div class="kelly-summary-item">' +
                    '<div class="kelly-summary-value">' + formatDollars(totalFull) + '</div>' +
                    '<div class="kelly-summary-label">Full Kelly Total</div>' +
                '</div>' +
                '<div class="kelly-summary-item">' +
                    '<div class="kelly-summary-value">' + formatDollars(totalHalf) + '</div>' +
                    '<div class="kelly-summary-label">\u00bd Kelly Total</div>' +
                '</div>' +
                '<div class="kelly-summary-item">' +
                    '<div class="kelly-summary-value">' + formatDollars(totalQuarter) + '</div>' +
                    '<div class="kelly-summary-label">\u00bc Kelly Total</div>' +
                '</div>' +
                '<div class="kelly-summary-item">' +
                    '<div class="kelly-summary-value">' + formatDollars(bankroll) + '</div>' +
                    '<div class="kelly-summary-label">Bankroll</div>' +
                '</div>';

            // Build correlation group summary
            var groups = {};
            combined.forEach(function(r) {
                var g = r.series || 'ungrouped';
                if (!groups[g]) groups[g] = { count: 0, totalKelly: 0 };
                groups[g].count++;
                groups[g].totalKelly += r.kelly;
            });
            var groupKeys = Object.keys(groups).sort();
            if (groupKeys.length > 1 || (groupKeys.length === 1 && groupKeys[0] !== 'ungrouped')) {
                var groupHtml = '<div style="margin-top:1rem;padding-top:1rem;border-top:1px solid rgba(255,255,255,0.1);">' +
                    '<div style="color:#93c5fd;font-size:0.875rem;font-weight:600;margin-bottom:0.5rem;">Correlation Groups (by Series)</div>' +
                    '<div style="display:flex;flex-wrap:wrap;gap:0.75rem;">';
                groupKeys.forEach(function(g) {
                    var pct = (groups[g].totalKelly * 100).toFixed(1);
                    var color = groups[g].totalKelly > 0.20 ? '#fca5a5' : '#86efac';
                    groupHtml += '<div style="background:rgba(255,255,255,0.05);padding:0.5rem 0.75rem;border-radius:6px;">' +
                        '<div style="font-size:0.7rem;color:#94a3b8;text-transform:uppercase;">' + g + '</div>' +
                        '<div style="font-weight:600;color:' + color + ';">' + groups[g].count + ' positions \u00b7 ' + pct + '% Kelly</div>' +
                    '</div>';
                });
                groupHtml += '</div></div>';
                kellySummary.innerHTML += groupHtml;
            }
        }

        function updateKellyPanel() {
            var combined = buildCombinedPortfolio();
            var bankroll = parseFloat(bankrollInput.value) || 0;

            if (combined.length === 0) {
                kellyPanel.classList.remove('visible');
                lastCombined = [];
                return;
            }

            kellyPanel.classList.add('visible');

            // Sort: CURRENT first, then NEW; within each group by edge descending
            combined.sort(function(a, b) {
                if (a.status !== b.status) return a.status === 'CURRENT' ? -1 : 1;
                return b.edge - a.edge;
            });

            lastCombined = combined;

            // Render all positions (positive and negative edge)
            var html = '';
            combined.forEach(function(r, idx) { html += renderRow(r, bankroll, idx); });
            kellyBody.innerHTML = html;

            renderSummary(combined, bankroll);
        }

        // Initial render (show existing positions immediately)
        updateKellyPanel();

        // Edge input delegation — recalculate row and summary on edge change
        document.addEventListener('input', function(e) {
            if (!e.target.classList.contains('edge-input')) return;
            var idx = parseInt(e.target.dataset.idx);
            var customEdgeCents = parseFloat(e.target.value);
            if (isNaN(customEdgeCents)) return;
            var customEdge = customEdgeCents / 100.0;
            var row = e.target.closest('tr');
            if (!row) return;
            if (idx >= lastCombined.length) return;
            var r = lastCombined[idx];
            // Persist the custom edge so it survives re-renders
            var key = r.ticker + '|' + r.side;
            customEdges[key] = customEdge;
            var bankroll = parseFloat(bankrollInput.value) || 0;
            var result = calculateKelly(r.ask, r.bid, customEdge);
            r.kelly = result.kelly;
            r.halfKelly = result.kelly / 2;
            r.quarterKelly = result.kelly / 4;
            r.edge = result.edge;
            r.customEdge = customEdge;
            // Update this row's Kelly cells
            var cells = row.querySelectorAll('td');
            var kellyClass = r.kelly > 0 ? 'kelly-positive' : 'kelly-zero';
            cells[6].className = kellyClass;
            cells[6].innerHTML = formatPercent(r.kelly) + ' \u00b7 ' + formatDollars(r.kelly * bankroll);
            cells[7].innerHTML = formatPercent(r.halfKelly) + ' \u00b7 ' + formatDollars(r.halfKelly * bankroll);
            cells[8].innerHTML = formatPercent(r.quarterKelly) + ' \u00b7 ' + formatDollars(r.quarterKelly * bankroll);
            // Recalculate summary totals
            renderSummary(lastCombined, bankroll);
        });

        // ===== Portfolio-level Kelly recalculation (proportional scaling) =====
        function recalcPortfolio() {
            var tbody = document.getElementById('portfolio-body');
            if (!tbody) return;
            var allRows = Array.from(tbody.querySelectorAll('.portfolio-row'));
            if (allRows.length === 0) return;
            // Compute individual raw Kelly for each row
            var rawKellys = [];
            allRows.forEach(function(row) {
                var ask = parseFloat(row.dataset.ask);
                var bid = parseFloat(row.dataset.bid);
                var edgeInput = row.querySelector('.portfolio-edge-input');
                var result;
                if (edgeInput && !isNaN(parseFloat(edgeInput.value))) {
                    result = calculateKelly(ask, bid, parseFloat(edgeInput.value) / 100.0);
                } else {
                    result = calculateKelly(ask, bid);
                }
                rawKellys.push(result.kelly);
            });
            // Sum positive Kelly fractions
            var totalRaw = 0;
            rawKellys.forEach(function(k) { if (k > 0) totalRaw += k; });
            // Proportional scaling: if total > 100%, scale down so total = 100%
            var scale = (totalRaw > 1.0) ? (1.0 / totalRaw) : 1.0;
            // Update each row's Kelly cells with scaled values
            allRows.forEach(function(row, i) {
                var kelly = rawKellys[i] * scale;
                var half = kelly / 2;
                var quarter = kelly / 4;
                var kf = row.querySelector('.pf-kelly-full');
                var kh = row.querySelector('.pf-kelly-half');
                var kq = row.querySelector('.pf-kelly-quarter');
                if (!kf) return;
                if (kelly > 0) {
                    kf.className = 'pf-kelly-full kelly-positive';
                    kf.textContent = formatPercent(kelly);
                    kh.textContent = formatPercent(half);
                    kq.textContent = formatPercent(quarter);
                } else {
                    kf.className = 'pf-kelly-full';
                    kf.textContent = '\u2014';
                    kh.textContent = '\u2014';
                    kq.textContent = '\u2014';
                }
            });
        }

        // Portfolio edge editing -> recalc entire portfolio
        document.addEventListener('input', function(e) {
            if (!e.target.classList.contains('portfolio-edge-input')) return;
            recalcPortfolio();
            // Also persist for Kelly panel
            var row = e.target.closest('tr');
            if (row) {
                var key = (row.dataset.ticker || '') + '|' + (row.dataset.side || '');
                var cents = parseFloat(e.target.value);
                if (!isNaN(cents)) customEdges[key] = cents / 100.0;
            }
            updateKellyPanel();
        });

        // ===== Checkbox-to-Portfolio: add/remove theoretical positions =====
        function addToPortfolio(market, skipRecalc) {
            var tbody = document.getElementById('portfolio-body');
            if (!tbody) return;
            var table = document.getElementById('portfolio-table');
            if (!table) return;
            // Avoid duplicates
            if (table.querySelector('tr.theoretical-row[data-ticker="' + market.ticker + '"][data-side="' + market.side + '"]')) return;
            var ask = market.ask;
            var bid = market.bid;
            var rawEdge = favoriteLongshotEdge(ask);
            var fee = kalshiFee(ask);
            var result = calculateKelly(ask, bid);
            var kellyStr = result.kelly > 0 ? formatPercent(result.kelly) : '\u2014';
            var halfStr = result.kelly > 0 ? formatPercent(result.kelly / 2) : '\u2014';
            var quarterStr = result.kelly > 0 ? formatPercent(result.kelly / 4) : '\u2014';
            var kellyClass = result.kelly > 0 ? 'pf-kelly-full kelly-positive' : 'pf-kelly-full';
            var sideClass = market.side === 'YES' ? 'side-yes' : 'side-no';
            var tr = document.createElement('tr');
            tr.className = 'portfolio-row theoretical-row';
            tr.dataset.ask = ask.toFixed(4);
            tr.dataset.bid = bid.toFixed(4);
            tr.dataset.ticker = market.ticker;
            tr.dataset.side = market.side;
            tr.dataset.series = market.series || '';
            tr.innerHTML =
                '<td><div class="market-title">' + market.title + ' <span class="new-badge">NEW</span></div><div class="market-subtitle">' + market.ticker + '</div></td>' +
                '<td><span class="side ' + sideClass + '">' + market.side + '</span></td>' +
                '<td>\u2014</td><td>\u2014</td>' +
                '<td>' + formatCents(ask) + '</td><td>' + formatCents(bid) + '</td>' +
                '<td>\u2014</td><td>\u2014</td>' +
                '<td>' + (market.expires || '\u2014') + '</td>' +
                '<td><input type="number" class="portfolio-edge-input" value="' + (rawEdge * 100).toFixed(1) + '" step="0.1" title="Fee: ' + (fee * 100).toFixed(1) + '\u00a2 auto-deducted">\u00a2</td>' +
                '<td class="' + kellyClass + '">' + kellyStr + '</td>' +
                '<td class="pf-kelly-half">' + halfStr + '</td>' +
                '<td class="pf-kelly-quarter">' + quarterStr + '</td>';
            tbody.appendChild(tr);
            if (!skipRecalc) recalcPortfolio();
        }

        function removeFromPortfolio(ticker, side, skipRecalc) {
            var table = document.getElementById('portfolio-table');
            if (!table) return;
            var row = table.querySelector('tr.theoretical-row[data-ticker="' + ticker + '"][data-side="' + side + '"]');
            if (row && row.parentNode) row.parentNode.removeChild(row);
            if (!skipRecalc) recalcPortfolio();
        }

        // Event listeners — with null checks for when markets table doesn't exist
        var selectAll = document.getElementById('select-all');
        var checkboxes = document.querySelectorAll('.market-select');

        if (selectAll) {
            selectAll.addEventListener('change', function() {
                checkboxes.forEach(function(cb) {
                    var wasChecked = cb.checked;
                    cb.checked = selectAll.checked;
                    var market = {
                        ticker: cb.dataset.ticker, title: cb.dataset.title,
                        side: cb.dataset.side, ask: parseFloat(cb.dataset.ask),
                        bid: parseFloat(cb.dataset.bid), series: cb.dataset.series || '',
                        expires: cb.dataset.expires || ''
                    };
                    if (cb.checked && !wasChecked) addToPortfolio(market, true);
                    else if (!cb.checked && wasChecked) removeFromPortfolio(market.ticker, market.side, true);
                });
                recalcPortfolio();
                updateKellyPanel();
            });
        }

        if (checkboxes.length > 0) {
            checkboxes.forEach(function(cb) {
                cb.addEventListener('change', function() {
                    var market = {
                        ticker: cb.dataset.ticker, title: cb.dataset.title,
                        side: cb.dataset.side, ask: parseFloat(cb.dataset.ask),
                        bid: parseFloat(cb.dataset.bid), series: cb.dataset.series || '',
                        expires: cb.dataset.expires || ''
                    };
                    if (cb.checked) addToPortfolio(market);
                    else removeFromPortfolio(market.ticker, market.side);
                    var allChecked = Array.from(checkboxes).every(function(c) { return c.checked; });
                    var someChecked = Array.from(checkboxes).some(function(c) { return c.checked; });
                    if (selectAll) {
                        selectAll.checked = allChecked;
                        selectAll.indeterminate = someChecked && !allChecked;
                    }
                    updateKellyPanel();
                });
            });
        }

        if (bankrollInput) bankrollInput.addEventListener('input', updateKellyPanel);

        // ===== Portfolio table sorting =====
        var portfolioTable = document.getElementById('portfolio-table');
        if (portfolioTable) {
            var pHeaders = portfolioTable.querySelectorAll('thead th');
            var pSortState = { column: -1, ascending: true };

            function getCellSortValue(cell) {
                if (cell.dataset && cell.dataset.value !== undefined && cell.dataset.value !== '') {
                    var dv = parseFloat(cell.dataset.value);
                    return isNaN(dv) ? cell.dataset.value : dv;
                }
                var input = cell.querySelector('input');
                if (input) return parseFloat(input.value) || 0;
                var text = cell.textContent.trim();
                if (text === '\u2014' || text === '\u2014') return -Infinity;
                var cleaned = text.replace(/[$\u00a2%,]/g, '').trim();
                var num = parseFloat(cleaned);
                return isNaN(num) ? text : num;
            }

            pHeaders.forEach(function(header, index) {
                header.addEventListener('click', function() {
                    var ascending = pSortState.column === index ? !pSortState.ascending : true;
                    var tbody = document.getElementById('portfolio-body');
                    if (!tbody) return;
                    var rows = Array.from(tbody.querySelectorAll('tr'));
                    rows.sort(function(a, b) {
                        var aVal = getCellSortValue(a.cells[index]);
                        var bVal = getCellSortValue(b.cells[index]);
                        var comparison;
                        if (typeof aVal === 'number' && typeof bVal === 'number') {
                            comparison = aVal - bVal;
                        } else {
                            comparison = String(aVal).localeCompare(String(bVal));
                        }
                        return ascending ? comparison : -comparison;
                    });
                    rows.forEach(function(row) { tbody.appendChild(row); });
                    pSortState = { column: index, ascending: ascending };
                    pHeaders.forEach(function(h) { h.classList.remove('sort-asc', 'sort-desc'); });
                    header.classList.add(ascending ? 'sort-asc' : 'sort-desc');
                });
            });
        }

        // ===== Transactions table sorting =====
        var txTable = document.getElementById('transactions-table');
        if (txTable) {
            var txHeaders = txTable.querySelectorAll('thead th');
            var txSortState = { column: -1, ascending: true };
            txHeaders.forEach(function(header, index) {
                header.addEventListener('click', function() {
                    var ascending = txSortState.column === index ? !txSortState.ascending : true;
                    var tbody = txTable.querySelector('tbody');
                    if (!tbody) return;
                    var rows = Array.from(tbody.querySelectorAll('tr'));
                    rows.sort(function(a, b) {
                        var aVal = getCellSortValue(a.cells[index]);
                        var bVal = getCellSortValue(b.cells[index]);
                        var comparison;
                        if (typeof aVal === 'number' && typeof bVal === 'number') {
                            comparison = aVal - bVal;
                        } else {
                            comparison = String(aVal).localeCompare(String(bVal));
                        }
                        return ascending ? comparison : -comparison;
                    });
                    rows.forEach(function(row) { tbody.appendChild(row); });
                    txSortState = { column: index, ascending: ascending };
                    txHeaders.forEach(function(h) { h.classList.remove('sort-asc', 'sort-desc'); });
                    header.classList.add(ascending ? 'sort-asc' : 'sort-desc');
                });
            });
        }

        // Initial portfolio recalculation (scale if existing positions already exceed 100%)
        recalcPortfolio();

        // ===== Hamburger menu and settings dialog =====
        var hamburgerBtn = document.getElementById('hamburger-btn');
        var menuDropdown = document.getElementById('menu-dropdown');
        var openSettingsBtn = document.getElementById('open-settings');
        var settingsOverlay = document.getElementById('settings-overlay');
        var settingsSave = document.getElementById('settings-save');
        var settingsCancel = document.getElementById('settings-cancel');
        var settingsStatus = document.getElementById('settings-status');

        if (hamburgerBtn) {
            hamburgerBtn.addEventListener('click', function(e) {
                e.stopPropagation();
                menuDropdown.classList.toggle('visible');
            });
        }

        // Close menu when clicking outside
        document.addEventListener('click', function(e) {
            if (menuDropdown && !menuDropdown.contains(e.target) && e.target !== hamburgerBtn) {
                menuDropdown.classList.remove('visible');
            }
        });

        if (openSettingsBtn) {
            openSettingsBtn.addEventListener('click', function() {
                menuDropdown.classList.remove('visible');
                settingsOverlay.classList.add('visible');
            });
        }

        if (settingsCancel) {
            settingsCancel.addEventListener('click', function() {
                settingsOverlay.classList.remove('visible');
            });
        }

        // Close overlay on background click
        if (settingsOverlay) {
            settingsOverlay.addEventListener('click', function(e) {
                if (e.target === settingsOverlay) {
                    settingsOverlay.classList.remove('visible');
                }
            });
        }

        if (settingsSave) {
            settingsSave.addEventListener('click', function() {
                var checkboxes = document.querySelectorAll('#category-list input[type="checkbox"]');
                var selected = [];
                checkboxes.forEach(function(cb) {
                    if (cb.checked) selected.push(cb.value);
                });

                if (selected.length === 0) {
                    alert('Please select at least one category.');
                    return;
                }

                // Try to save to server (when served by kalshi-serve)
                settingsStatus.style.display = 'block';
                settingsStatus.textContent = 'Saving settings...';

                fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ categories: selected })
                }).then(function(resp) {
                    if (!resp.ok) throw new Error('Save failed');
                    settingsStatus.textContent = 'Settings saved! Starting scan...';
                    // Trigger refresh
                    return fetch('/api/refresh', { method: 'POST' });
                }).then(function(resp) {
                    if (!resp.ok) throw new Error('Refresh failed');
                    settingsStatus.textContent = 'Scan started. Page will reload when ready...';
                    // Poll for new report
                    setTimeout(function() { location.reload(); }, 5000);
                }).catch(function(err) {
                    // If not served by kalshi-serve, save to localStorage instead
                    localStorage.setItem('kalshi-categories', JSON.stringify(selected));
                    settingsStatus.textContent = 'Saved to browser. Run scanner with updated categories to apply.';
                    settingsStatus.style.color = 'var(--warning)';

                    // Apply client-side filtering immediately
                    filterMarketsByCategory(selected);
                    setTimeout(function() {
                        settingsOverlay.classList.remove('visible');
                        settingsStatus.style.display = 'none';
                        settingsStatus.style.color = 'var(--success)';
                    }, 2000);
                });
            });
        }

        // Client-side category filtering
        function filterMarketsByCategory(categories) {
            var rows = document.querySelectorAll('#markets-table tbody tr');
            rows.forEach(function(row) {
                var cat = row.dataset.category || '';
                if (categories.length === 0 || categories.indexOf(cat) !== -1 || cat === '') {
                    row.style.display = '';
                } else {
                    row.style.display = 'none';
                    // Uncheck hidden rows
                    var cb = row.querySelector('.market-select');
                    if (cb && cb.checked) {
                        cb.checked = false;
                        removeFromPortfolio(cb.dataset.ticker, cb.dataset.side);
                    }
                }
            });
        }

        // Apply saved category filter from localStorage on page load
        (function() {
            var saved = localStorage.getItem('kalshi-categories');
            if (saved) {
                try {
                    var cats = JSON.parse(saved);
                    if (Array.isArray(cats) && cats.length > 0) {
                        filterMarketsByCategory(cats);
                    }
                } catch(e) {}
            }
        })();
    })();
    </script>
</body>
</html>`

// EnrichedPosition contains a portfolio position with resolved title and Kelly values.
type EnrichedPosition struct {
	Position             models.MarketPosition
	Title                string
	SeriesTicker         string
	ExpirationStr        string
	ExpirationUnix       int64
	AskPrice             float64
	BidPrice             float64
	Edge                 float64
	KellyFraction        float64
	HalfKellyFraction    float64
	QuarterKellyFraction float64
}

// PortfolioPositionView contains a portfolio position formatted for the report.
type PortfolioPositionView struct {
	Ticker          string
	Title           string
	Side            string
	Contracts       int
	PurchasePrice   string
	CurrentAskCents string
	CurrentBidCents string
	Exposure        string
	UnrealizedPnL   string
	ExpirationStr   string
	ExpirationUnix  int64
	EdgeStr         string
	KellyStr        string
	HalfKellyStr    string
	QuarterKellyStr string
	// For JS interactivity
	AskPrice     float64
	BidPrice     float64
	RawEdgeCents float64 // Raw bias edge before fee deduction, in cents
	FeeCents     float64 // Kalshi taker fee in cents
	SeriesTicker string  // For correlation grouping
}

// positionJSON is the format embedded in the HTML for JavaScript access.
type positionJSON struct {
	Ticker string  `json:"ticker"`
	Title  string  `json:"title"`
	Side   string  `json:"side"`
	Ask    float64 `json:"ask"`
	Bid    float64 `json:"bid"`
	Series string  `json:"series"`
}

// TransactionView contains a settlement or sale formatted for the report.
type TransactionView struct {
	Ticker      string
	Title       string
	Type        string // "Settlement" or "Sale"
	Side        string // "YES" or "NO"
	Contracts   int
	Date        string
	DateUnix    int64
	CostStr     string  // Purchase price display
	RevenueStr  string  // Settlement/sale price display
	GainLoss    float64 // For conditional coloring
	GainLossStr string
	FeesStr     string
	TaxStr      string
}

// CategoryView represents a category for the settings dialog.
type CategoryView struct {
	Name   string
	Active bool // Was this category included in the scan?
}

// ReportData contains data for the HTML report.
type ReportData struct {
	GeneratedAt     string
	TotalContracts  int
	TotalSeries     int
	MinPrice        float64 // In cents
	MaxPrice        float64 // In cents
	MinPriceDollars float64
	MaxPriceDollars float64
	Markets         []ReportMarket

	// Portfolio data
	HasPortfolio       bool
	CashBalance        float64 // In dollars
	PortfolioValue     float64 // In dollars
	TotalBankroll      float64 // Cash + portfolio value in dollars
	NumPositions       int
	PortfolioPositions []PortfolioPositionView
	PositionsJSON      template.JS // JSON for JavaScript consumption

	// Transaction data
	Transactions     []TransactionView
	TransactionYear  int
	TransactionCount int
	TotalGainLoss    float64
	TotalFees        float64
	TotalTax         float64

	// Category settings
	AllCategories    []CategoryView
	ActiveCategories string // Comma-separated for JS
}

// ReportMarket contains market data formatted for the report.
type ReportMarket struct {
	Market        FilteredMarket
	Index         int // Row index for checkbox identification
	Side          string
	AskCents      float64
	BidCents      float64
	AskDollars    float64
	BidDollars    float64
	SpreadCents   float64
	ExpirationStr string
	ExpiresSoon   bool   // Within 7 days
	Category      string // Source category (e.g., "Politics")
}

// GenerateHTMLReport creates an HTML report and saves it to the given path.
// If balance and enriched positions are provided, the report includes portfolio data
// and Kelly Criterion recommendations.
func GenerateHTMLReport(
	markets []FilteredMarket,
	totalSeries int,
	minPrice, maxPrice float64,
	outputPath string,
	balance *models.GetBalanceResponse,
	enrichedPositions []EnrichedPosition,
	settlements []models.Settlement,
	fills []models.Fill,
	marketTitles map[string]string,
	categoryViews []CategoryView,
) error {
	// Prepare report data
	reportMarkets := make([]ReportMarket, len(markets))
	now := time.Now()
	sevenDaysFromNow := now.AddDate(0, 0, 7)

	for i, m := range markets {
		expirationStr := "N/A"
		expiresSoon := false
		if !m.Expiration.IsZero() {
			expirationStr = m.Expiration.Format("Jan 2, 2006")
			expiresSoon = m.Expiration.Before(sevenDaysFromNow)
		}

		reportMarkets[i] = ReportMarket{
			Market:        m,
			Index:         i,
			Side:          string(m.Side),
			AskCents:      m.AskPrice * 100,
			BidCents:      m.BidPrice * 100,
			AskDollars:    m.AskPrice,
			BidDollars:    m.BidPrice,
			SpreadCents:   (m.AskPrice - m.BidPrice) * 100,
			ExpirationStr: expirationStr,
			ExpiresSoon:   expiresSoon,
			Category:      m.Category,
		}
	}

	data := ReportData{
		GeneratedAt:     time.Now().Format("January 2, 2006 at 3:04 PM MST"),
		TotalContracts:  len(markets),
		TotalSeries:     totalSeries,
		MinPrice:        minPrice * 100,
		MaxPrice:        maxPrice * 100,
		MinPriceDollars: minPrice,
		MaxPriceDollars: maxPrice,
		Markets:         reportMarkets,
	}

	data.AllCategories = categoryViews
	// Build comma-separated list of active categories for JS
	var activeNames []string
	for _, cv := range categoryViews {
		if cv.Active {
			activeNames = append(activeNames, cv.Name)
		}
	}
	data.ActiveCategories = strings.Join(activeNames, ", ")

	// Add portfolio data if available
	if balance != nil {
		cashDollars := float64(balance.Balance) / 100.0
		portfolioDollars := float64(balance.PortfolioValue) / 100.0

		data.HasPortfolio = true
		data.CashBalance = cashDollars
		data.PortfolioValue = portfolioDollars
		data.TotalBankroll = cashDollars + portfolioDollars
		data.NumPositions = len(enrichedPositions)

		// Build portfolio position views and JSON data for JS
		var jsPositions []positionJSON

		for _, ep := range enrichedPositions {
			p := ep.Position
			side := "YES"
			contracts := p.Position
			if p.Position < 0 {
				side = "NO"
				contracts = -p.Position
			}

			var avgPriceDollars float64
			if contracts > 0 {
				avgPriceDollars = float64(p.MarketExposure) / 100.0 / float64(contracts)
			}
			purchasePrice := fmt.Sprintf("%.0f¢", avgPriceDollars*100)
			currentAskCents := fmt.Sprintf("%.0f¢", ep.AskPrice*100)
			currentBidCents := fmt.Sprintf("%.0f¢", ep.BidPrice*100)

			// Unrealized P&L = (current bid - avg price) × contracts
			unrealizedPnl := (ep.BidPrice - avgPriceDollars) * float64(contracts)
			unrealizedStr := fmt.Sprintf("$%.2f", unrealizedPnl)

			exposure := p.MarketExposureDollars
			if exposure == "" {
				exposure = fmt.Sprintf("$%.2f", float64(p.MarketExposure)/100.0)
			} else {
				if val, err := strconv.ParseFloat(exposure, 64); err == nil {
					exposure = fmt.Sprintf("$%.2f", val)
				}
			}

			// Format Kelly strings
			rawEdge := FavoriteLongshotEdge(ep.AskPrice)
			fee := KalshiFee(ep.AskPrice)
			edgeStr := fmt.Sprintf("%.1f¢", ep.Edge*100)
			kellyStr := fmt.Sprintf("%.1f%%", ep.KellyFraction*100)
			halfKellyStr := fmt.Sprintf("%.1f%%", ep.HalfKellyFraction*100)
			quarterKellyStr := fmt.Sprintf("%.1f%%", ep.QuarterKellyFraction*100)
			if ep.KellyFraction <= 0 {
				kellyStr = "—"
				halfKellyStr = "—"
				quarterKellyStr = "—"
			}

			data.PortfolioPositions = append(data.PortfolioPositions, PortfolioPositionView{
				Ticker:          p.Ticker,
				Title:           ep.Title,
				Side:            side,
				Contracts:       contracts,
				PurchasePrice:   purchasePrice,
				CurrentAskCents: currentAskCents,
				CurrentBidCents: currentBidCents,
				Exposure:        exposure,
				UnrealizedPnL:   unrealizedStr,
				ExpirationStr:   ep.ExpirationStr,
				ExpirationUnix:  ep.ExpirationUnix,
				EdgeStr:         edgeStr,
				KellyStr:        kellyStr,
				HalfKellyStr:    halfKellyStr,
				QuarterKellyStr: quarterKellyStr,
				AskPrice:        ep.AskPrice,
				BidPrice:        ep.BidPrice,
				RawEdgeCents:    rawEdge * 100,
				FeeCents:        fee * 100,
				SeriesTicker:    ep.SeriesTicker,
			})

			jsPositions = append(jsPositions, positionJSON{
				Ticker: p.Ticker,
				Title:  ep.Title,
				Side:   side,
				Ask:    ep.AskPrice,
				Bid:    ep.BidPrice,
				Series: ep.SeriesTicker,
			})
		}

		// Serialize positions to JSON for JavaScript
		jsonBytes, err := json.Marshal(jsPositions)
		if err != nil {
			data.PositionsJSON = template.JS("[]")
		} else {
			data.PositionsJSON = template.JS(jsonBytes)
		}
	} else {
		data.PositionsJSON = template.JS("[]")
	}

	// Build transaction views from settlements and sell-fills
	if len(settlements) > 0 || len(fills) > 0 {
		data.TransactionYear = time.Now().Year()

		// Process settlements
		for _, s := range settlements {
			title := s.Ticker
			if t, ok := marketTitles[s.Ticker]; ok {
				title = t
			}

			// Determine side and cost basis
			side := "YES"
			contracts := s.YesCount
			costCents := s.YesTotalCost
			if s.NoCount > 0 && s.YesCount == 0 {
				side = "NO"
				contracts = s.NoCount
				costCents = s.NoTotalCost
			}

			costDollars := float64(costCents) / 100.0
			revenueDollars := float64(s.Revenue) / 100.0

			// Parse fee (FixedPointDollars string like "0.3400")
			feeDollars := 0.0
			if s.FeeCost != "" {
				feeDollars, _ = strconv.ParseFloat(s.FeeCost, 64)
			}

			gainLoss := revenueDollars - costDollars - feeDollars
			tax := 0.0
			if gainLoss > 0 {
				tax = gainLoss * 0.21
			}

			data.Transactions = append(data.Transactions, TransactionView{
				Ticker:      s.Ticker,
				Title:       title,
				Type:        "Settlement",
				Side:        side,
				Contracts:   contracts,
				Date:        parseTime(s.SettledTime).Format("Jan 2, 2006"),
				DateUnix:    parseTime(s.SettledTime).Unix(),
				CostStr:     fmt.Sprintf("$%.2f", costDollars),
				RevenueStr:  fmt.Sprintf("$%.2f", revenueDollars),
				GainLoss:    gainLoss,
				GainLossStr: fmt.Sprintf("$%.2f", gainLoss),
				FeesStr:     fmt.Sprintf("$%.2f", feeDollars),
				TaxStr:      fmt.Sprintf("$%.2f", tax),
			})

			data.TotalGainLoss += gainLoss
			data.TotalFees += feeDollars
			data.TotalTax += tax
		}

		// Process sell fills
		for _, f := range fills {
			if f.Action != "sell" {
				continue
			}

			title := f.Ticker
			if t, ok := marketTitles[f.Ticker]; ok {
				title = t
			}

			side := "YES"
			priceCents := f.YesPrice
			if f.Side == "no" {
				side = "NO"
				priceCents = f.NoPrice
			}

			revenueDollars := float64(priceCents) * float64(f.Count) / 100.0

			feeDollars := 0.0
			if f.FeeCost != "" {
				feeDollars, _ = strconv.ParseFloat(f.FeeCost, 64)
			}

			// For sells, we don't have the original purchase price from the fill.
			// Show the sale revenue and fee; gain/loss is revenue - fees.
			gainLoss := revenueDollars - feeDollars
			tax := 0.0
			if gainLoss > 0 {
				tax = gainLoss * 0.21
			}

			data.Transactions = append(data.Transactions, TransactionView{
				Ticker:      f.Ticker,
				Title:       title,
				Type:        "Sale",
				Side:        side,
				Contracts:   f.Count,
				Date:        parseTime(f.CreatedTime).Format("Jan 2, 2006"),
				DateUnix:    parseTime(f.CreatedTime).Unix(),
				CostStr:     "—",
				RevenueStr:  fmt.Sprintf("$%.2f", revenueDollars),
				GainLoss:    gainLoss,
				GainLossStr: fmt.Sprintf("$%.2f", gainLoss),
				FeesStr:     fmt.Sprintf("$%.2f", feeDollars),
				TaxStr:      fmt.Sprintf("$%.2f", tax),
			})

			data.TotalGainLoss += gainLoss
			data.TotalFees += feeDollars
			data.TotalTax += tax
		}

		data.TransactionCount = len(data.Transactions)
	}

	// Parse and execute template
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// GenerateHTMLReportString produces the same report as GenerateHTMLReport but
// returns the HTML as a string instead of writing it to a file.
func GenerateHTMLReportString(
	markets []FilteredMarket,
	totalSeries int,
	minPrice, maxPrice float64,
	balance *models.GetBalanceResponse,
	enrichedPositions []EnrichedPosition,
	settlements []models.Settlement,
	fills []models.Fill,
	marketTitles map[string]string,
	categoryViews []CategoryView,
) (string, error) {
	// Prepare report data
	reportMarkets := make([]ReportMarket, len(markets))
	now := time.Now()
	sevenDaysFromNow := now.AddDate(0, 0, 7)

	for i, m := range markets {
		expirationStr := "N/A"
		expiresSoon := false
		if !m.Expiration.IsZero() {
			expirationStr = m.Expiration.Format("Jan 2, 2006")
			expiresSoon = m.Expiration.Before(sevenDaysFromNow)
		}

		reportMarkets[i] = ReportMarket{
			Market:        m,
			Index:         i,
			Side:          string(m.Side),
			AskCents:      m.AskPrice * 100,
			BidCents:      m.BidPrice * 100,
			AskDollars:    m.AskPrice,
			BidDollars:    m.BidPrice,
			SpreadCents:   (m.AskPrice - m.BidPrice) * 100,
			ExpirationStr: expirationStr,
			ExpiresSoon:   expiresSoon,
			Category:      m.Category,
		}
	}

	data := ReportData{
		GeneratedAt:     time.Now().Format("January 2, 2006 at 3:04 PM MST"),
		TotalContracts:  len(markets),
		TotalSeries:     totalSeries,
		MinPrice:        minPrice * 100,
		MaxPrice:        maxPrice * 100,
		MinPriceDollars: minPrice,
		MaxPriceDollars: maxPrice,
		Markets:         reportMarkets,
	}

	data.AllCategories = categoryViews
	var activeNames []string
	for _, cv := range categoryViews {
		if cv.Active {
			activeNames = append(activeNames, cv.Name)
		}
	}
	data.ActiveCategories = strings.Join(activeNames, ", ")

	// Add portfolio data if available
	if balance != nil {
		cashDollars := float64(balance.Balance) / 100.0
		portfolioDollars := float64(balance.PortfolioValue) / 100.0

		data.HasPortfolio = true
		data.CashBalance = cashDollars
		data.PortfolioValue = portfolioDollars
		data.TotalBankroll = cashDollars + portfolioDollars
		data.NumPositions = len(enrichedPositions)

		var jsPositions []positionJSON
		for _, ep := range enrichedPositions {
			p := ep.Position
			side := "YES"
			contracts := p.Position
			if p.Position < 0 {
				side = "NO"
				contracts = -p.Position
			}
			var avgPriceDollars float64
			if contracts > 0 {
				avgPriceDollars = float64(p.MarketExposure) / 100.0 / float64(contracts)
			}
			purchasePrice := fmt.Sprintf("%.0f¢", avgPriceDollars*100)
			currentAskCents := fmt.Sprintf("%.0f¢", ep.AskPrice*100)
			currentBidCents := fmt.Sprintf("%.0f¢", ep.BidPrice*100)
			unrealizedPnl := (ep.BidPrice - avgPriceDollars) * float64(contracts)
			unrealizedStr := fmt.Sprintf("$%.2f", unrealizedPnl)
			exposure := p.MarketExposureDollars
			if exposure == "" {
				exposure = fmt.Sprintf("$%.2f", float64(p.MarketExposure)/100.0)
			} else {
				if val, err := strconv.ParseFloat(exposure, 64); err == nil {
					exposure = fmt.Sprintf("$%.2f", val)
				}
			}
			rawEdge := FavoriteLongshotEdge(ep.AskPrice)
			fee := KalshiFee(ep.AskPrice)
			edgeStr := fmt.Sprintf("%.1f¢", ep.Edge*100)
			kellyStr := fmt.Sprintf("%.1f%%", ep.KellyFraction*100)
			halfKellyStr := fmt.Sprintf("%.1f%%", ep.HalfKellyFraction*100)
			quarterKellyStr := fmt.Sprintf("%.1f%%", ep.QuarterKellyFraction*100)
			if ep.KellyFraction <= 0 {
				kellyStr = "—"
				halfKellyStr = "—"
				quarterKellyStr = "—"
			}
			data.PortfolioPositions = append(data.PortfolioPositions, PortfolioPositionView{
				Ticker: p.Ticker, Title: ep.Title, Side: side, Contracts: contracts,
				PurchasePrice: purchasePrice, CurrentAskCents: currentAskCents,
				CurrentBidCents: currentBidCents, Exposure: exposure,
				UnrealizedPnL: unrealizedStr, ExpirationStr: ep.ExpirationStr,
				ExpirationUnix: ep.ExpirationUnix, EdgeStr: edgeStr,
				KellyStr: kellyStr, HalfKellyStr: halfKellyStr,
				QuarterKellyStr: quarterKellyStr, AskPrice: ep.AskPrice,
				BidPrice: ep.BidPrice, RawEdgeCents: rawEdge * 100,
				FeeCents: fee * 100, SeriesTicker: ep.SeriesTicker,
			})
			jsPositions = append(jsPositions, positionJSON{
				Ticker: p.Ticker, Title: ep.Title, Side: side,
				Ask: ep.AskPrice, Bid: ep.BidPrice, Series: ep.SeriesTicker,
			})
		}
		jsonBytes, err := json.Marshal(jsPositions)
		if err != nil {
			data.PositionsJSON = template.JS("[]")
		} else {
			data.PositionsJSON = template.JS(jsonBytes)
		}
	} else {
		data.PositionsJSON = template.JS("[]")
	}

	// Build transaction views
	if len(settlements) > 0 || len(fills) > 0 {
		data.TransactionYear = time.Now().Year()
		for _, s := range settlements {
			title := s.Ticker
			if t, ok := marketTitles[s.Ticker]; ok {
				title = t
			}
			side := "YES"
			contracts := s.YesCount
			costCents := s.YesTotalCost
			if s.NoCount > 0 && s.YesCount == 0 {
				side = "NO"
				contracts = s.NoCount
				costCents = s.NoTotalCost
			}
			costDollars := float64(costCents) / 100.0
			revenueDollars := float64(s.Revenue) / 100.0
			feeDollars := 0.0
			if s.FeeCost != "" {
				feeDollars, _ = strconv.ParseFloat(s.FeeCost, 64)
			}
			gainLoss := revenueDollars - costDollars - feeDollars
			tax := 0.0
			if gainLoss > 0 {
				tax = gainLoss * 0.21
			}
			data.Transactions = append(data.Transactions, TransactionView{
				Ticker: s.Ticker, Title: title, Type: "Settlement", Side: side,
				Contracts: contracts, Date: parseTime(s.SettledTime).Format("Jan 2, 2006"),
				DateUnix:   parseTime(s.SettledTime).Unix(),
				CostStr:    fmt.Sprintf("$%.2f", costDollars),
				RevenueStr: fmt.Sprintf("$%.2f", revenueDollars),
				GainLoss:   gainLoss, GainLossStr: fmt.Sprintf("$%.2f", gainLoss),
				FeesStr: fmt.Sprintf("$%.2f", feeDollars), TaxStr: fmt.Sprintf("$%.2f", tax),
			})
			data.TotalGainLoss += gainLoss
			data.TotalFees += feeDollars
			data.TotalTax += tax
		}
		for _, f := range fills {
			if f.Action != "sell" {
				continue
			}
			title := f.Ticker
			if t, ok := marketTitles[f.Ticker]; ok {
				title = t
			}
			side := "YES"
			priceCents := f.YesPrice
			if f.Side == "no" {
				side = "NO"
				priceCents = f.NoPrice
			}
			revenueDollars := float64(priceCents) * float64(f.Count) / 100.0
			feeDollars := 0.0
			if f.FeeCost != "" {
				feeDollars, _ = strconv.ParseFloat(f.FeeCost, 64)
			}
			gainLoss := revenueDollars - feeDollars
			tax := 0.0
			if gainLoss > 0 {
				tax = gainLoss * 0.21
			}
			data.Transactions = append(data.Transactions, TransactionView{
				Ticker: f.Ticker, Title: title, Type: "Sale", Side: side,
				Contracts: f.Count, Date: parseTime(f.CreatedTime).Format("Jan 2, 2006"),
				DateUnix:    parseTime(f.CreatedTime).Unix(),
				CostStr:     "—",
				RevenueStr:  fmt.Sprintf("$%.2f", revenueDollars),
				GainLoss:    gainLoss,
				GainLossStr: fmt.Sprintf("$%.2f", gainLoss),
				FeesStr:     fmt.Sprintf("$%.2f", feeDollars),
				TaxStr:      fmt.Sprintf("$%.2f", tax),
			})
			data.TotalGainLoss += gainLoss
			data.TotalFees += feeDollars
			data.TotalTax += tax
		}
		data.TransactionCount = len(data.Transactions)
	}

	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
