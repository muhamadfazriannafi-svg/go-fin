import React, { useEffect, useState } from 'react';
import { createClient } from '@supabase/supabase-js';

// Inisialisasi Supabase
const supabaseUrl = import.meta.env.VITE_SUPABASE_URL;
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY;
const supabase = createClient(supabaseUrl, supabaseAnonKey);

export default function App() {
  const [transactions, setTransactions] = useState([]);
  const [balance, setBalance] = useState(0);

  useEffect(() => {
    fetchTransactions();
    
    // Auto-refresh saat ada data baru dari Telegram
    const channel = supabase
      .channel('realtime-transactions')
      .on('postgres_changes', { event: 'INSERT', schema: 'public', table: 'transactions' }, () => {
        fetchTransactions();
      })
      .subscribe();

    return () => { supabase.removeChannel(channel); };
  }, []);

  async function fetchTransactions() {
    const { data, error } = await supabase
      .from('transactions')
      .select('*')
      .order('transaction_date', { ascending: false });

    if (error) console.error("Error fetching:", error);
    else {
      setTransactions(data);
      // Hitung Saldo Awal / Total
      const total = data.reduce((acc, curr) => {
        return curr.type === 'income' ? acc + curr.amount : acc - curr.amount;
      }, 0);
      setBalance(total);
    }
  }

  return (
    <div style={{ padding: '40px', fontFamily: 'system-ui, sans-serif', maxWidth: '800px', margin: '0 auto' }}>
      <h1 style={{ color: '#0ea5e9' }}>🚀 GoFin Dashboard</h1>
      
      <div style={{ background: '#f0f9ff', padding: '20px', borderRadius: '12px', marginBottom: '30px' }}>
        <h2 style={{ margin: 0, color: '#333' }}>Saldo Saat Ini</h2>
        <h1 style={{ margin: '10px 0 0 0', fontSize: '2.5rem', color: balance >= 0 ? '#16a34a' : '#dc2626' }}>
          Rp {balance.toLocaleString('id-ID')}
        </h1>
      </div>

      <h3>Riwayat Transaksi Terbaru</h3>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
        {transactions.map((tx) => (
          <div key={tx.id} style={{ 
            display: 'flex', justifyContent: 'space-between', alignItems: 'center', 
            padding: '15px', border: '1px solid #e5e7eb', borderRadius: '8px', background: 'white' 
          }}>
            <div>
              <p style={{ margin: '0 0 5px 0', fontWeight: 'bold' }}>{tx.description}</p>
              <span style={{ fontSize: '12px', background: '#e2e8f0', padding: '4px 8px', borderRadius: '4px' }}>
                {tx.category}
              </span>
              <span style={{ fontSize: '12px', color: '#64748b', marginLeft: '10px' }}>
                {new Date(tx.transaction_date).toLocaleString('id-ID')}
              </span>
            </div>
            <div>
              <h3 style={{ margin: 0, color: tx.type === 'income' ? '#16a34a' : '#dc2626' }}>
                {tx.type === 'income' ? '+' : '-'} Rp {tx.amount.toLocaleString('id-ID')}
              </h3>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}