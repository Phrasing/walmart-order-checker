import { useState } from 'react';

export function ReportView({ data }) {
  const [searchTerm, setSearchTerm] = useState('');

  if (!data || !data.orders) {
    return null;
  }

  const orders = Object.values(data.orders);
  const totalOrders = orders.length;
  const canceledOrders = orders.filter(o => o.Status === 'canceled').length;
  const liveOrderCount = data.email_stats?.LiveOrderCount || 0;
  const cancellationRate = data.email_stats?.CancellationRate || 0;

  const filterRows = (rows) => {
    if (!searchTerm) return rows;
    const term = searchTerm.toLowerCase();
    return rows.filter(row =>
      JSON.stringify(row).toLowerCase().includes(term)
    );
  };

  const liveOrderSummary = filterRows(data.live_order_summary || []);
  const liveOrders = filterRows(data.live_orders || []);
  const productCancel = filterRows(data.product_cancel || []);
  const orderLines = filterRows(data.order_lines || []);
  const productSpend = filterRows(data.product_spend || []);
  const shipments = filterRows(data.shipments || []);

  return (
    <div className="space-y-6">
      {/* Header with Search */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-semibold mb-1">Walmart Order Checker</h1>
          <p className="text-muted text-sm">{data.date_range || 'Order Report'}</p>
        </div>
        <div>
          <input
            type="text"
            placeholder="Search products..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="px-4 py-2 bg-panel border border-muted/20 rounded-lg text-sm focus:outline-none focus:border-primary transition-colors max-w-xs"
          />
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-panel rounded-xl p-5 border border-muted/10 shadow-sm text-center">
          <div className="text-3xl mb-2">✅</div>
          <div className="text-muted text-xs uppercase tracking-wider mb-2">Live Order Count</div>
          <div className="text-2xl font-semibold text-primary font-mono">{liveOrderCount}</div>
        </div>

        <div className="bg-panel rounded-xl p-5 border border-muted/10 shadow-sm text-center">
          <div className="text-3xl mb-2">📦</div>
          <div className="text-muted text-xs uppercase tracking-wider mb-2">Total Unique Orders</div>
          <div className="text-2xl font-semibold text-primary font-mono">{totalOrders}</div>
        </div>

        <div className="bg-panel rounded-xl p-5 border border-muted/10 shadow-sm text-center">
          <div className="text-3xl mb-2">❌</div>
          <div className="text-muted text-xs uppercase tracking-wider mb-2">Total Canceled Orders</div>
          <div className="text-2xl font-semibold text-primary font-mono">{canceledOrders}</div>
        </div>

        <div className="bg-panel rounded-xl p-5 border border-muted/10 shadow-sm text-center">
          <div className="text-3xl mb-2">📊</div>
          <div className="text-muted text-xs uppercase tracking-wider mb-2">Cancellation Rate</div>
          <div className="text-2xl font-semibold text-primary font-mono">{cancellationRate.toFixed(2)}%</div>
        </div>
      </div>

      {/* Live Order Summary */}
      {liveOrderSummary.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10 flex items-center justify-between">
            <div>
              <h2 className="text-lg font-semibold">Live Order Summary</h2>
              <p className="text-muted text-xs mt-0.5">Total units and estimated spend for live orders</p>
            </div>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider"></th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Product Name</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Total Units</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Price / Unit</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Estimated Spend</th>
                </tr>
              </thead>
              <tbody>
                {liveOrderSummary.map((item, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3">
                      <img src={item.Thumbnail} alt="" className="w-10 h-10 rounded-lg object-cover bg-bg border border-muted/10" />
                    </td>
                    <td className="px-4 py-3 text-sm">{item.Name}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{item.TotalUnits}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">
                      {item.PricePerUnit > 0 ? `$${item.PricePerUnit.toFixed(2)}` : '—'}
                    </td>
                    <td className="px-4 py-3 text-sm text-right font-mono">
                      {item.TotalSpent > 0 ? `$${item.TotalSpent.toFixed(2)}` : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Live Orders */}
      {liveOrders.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10">
            <h2 className="text-lg font-semibold">Live Orders</h2>
            <p className="text-muted text-xs mt-0.5">Confirmed orders awaiting shipment</p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Order Date</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Order #</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider"></th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Product Name</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Quantity</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Status</th>
                </tr>
              </thead>
              <tbody>
                {liveOrders.map((order, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3 text-sm font-mono">{order.OrderDate}</td>
                    <td className="px-4 py-3 text-sm font-mono">{order.OrderID}</td>
                    <td className="px-4 py-3">
                      <img src={order.Thumbnail} alt="" className="w-10 h-10 rounded-lg object-cover bg-bg border border-muted/10" />
                    </td>
                    <td className="px-4 py-3 text-sm">{order.Name}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{order.Quantity}</td>
                    <td className="px-4 py-3 text-right">
                      <span className="inline-flex px-2 py-1 text-xs font-medium rounded-full border border-success/20 bg-success/10 text-success">
                        Confirmed
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Product Cancellation */}
      {productCancel.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10">
            <h2 className="text-lg font-semibold">Product Cancellation</h2>
            <p className="text-muted text-xs mt-0.5">Rates by product</p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider"></th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Product Name</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Total Ordered</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Total Canceled</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Cancel Rate</th>
                </tr>
              </thead>
              <tbody>
                {productCancel.map((product, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3">
                      <img src={product.Thumbnail} alt="" className="w-10 h-10 rounded-lg object-cover bg-bg border border-muted/10" />
                    </td>
                    <td className="px-4 py-3 text-sm">{product.Name}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{product.TotalOrdered}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{product.TotalCanceled}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{product.CancelRate.toFixed(2)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Order Lines */}
      {orderLines.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10">
            <h2 className="text-lg font-semibold">Order Lines</h2>
            <p className="text-muted text-xs mt-0.5">Individual line items</p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Order Date</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Order #</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider"></th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Product Name</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Quantity</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Line Total (Est.)</th>
                </tr>
              </thead>
              <tbody>
                {orderLines.map((line, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3 text-sm font-mono">{line.OrderDate}</td>
                    <td className="px-4 py-3 text-sm font-mono">{line.OrderID}</td>
                    <td className="px-4 py-3">
                      <img src={line.Thumbnail} alt="" className="w-10 h-10 rounded-lg object-cover bg-bg border border-muted/10" />
                    </td>
                    <td className="px-4 py-3 text-sm">{line.Name}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{line.Quantity}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{line.Total}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Product Spend */}
      {productSpend.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10">
            <h2 className="text-lg font-semibold">Product Spend (Estimated)</h2>
            <p className="text-muted text-xs mt-0.5">Orders with a single product only</p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider"></th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Product Name</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Total Units</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Price / Unit</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Total Spent</th>
                </tr>
              </thead>
              <tbody>
                {productSpend.map((product, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3">
                      <img src={product.Thumbnail} alt="" className="w-10 h-10 rounded-lg object-cover bg-bg border border-muted/10" />
                    </td>
                    <td className="px-4 py-3 text-sm">{product.Name}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">{product.TotalUnits}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">${product.PricePerUnit.toFixed(2)}</td>
                    <td className="px-4 py-3 text-sm text-right font-mono">${product.TotalSpent.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Shipments */}
      {shipments.length > 0 && (
        <section className="bg-panel rounded-xl border border-muted/10 shadow-sm overflow-hidden">
          <div className="px-5 py-4 border-b border-muted/10">
            <h2 className="text-lg font-semibold">Shipments</h2>
            <p className="text-muted text-xs mt-0.5">Tracking overview</p>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="border-b border-muted/10">
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Order #</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Carrier</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Tracking #</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-muted uppercase tracking-wider">Estimated Arrival</th>
                </tr>
              </thead>
              <tbody>
                {shipments.map((shipment, idx) => (
                  <tr key={idx} className="border-b border-muted/10 hover:bg-primary/5 transition-colors">
                    <td className="px-4 py-3 text-sm font-mono">{shipment.ID}</td>
                    <td className="px-4 py-3 text-sm">{shipment.Carrier}</td>
                    <td className="px-4 py-3 text-sm font-mono">{shipment.TrackingNumber}</td>
                    <td className="px-4 py-3 text-sm font-mono">{shipment.EstimatedArrival}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
}
