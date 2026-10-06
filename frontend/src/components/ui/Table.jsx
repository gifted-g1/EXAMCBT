import './Table.css'

export default function Table({ columns, rows, onRowClick, emptyMessage = 'No records yet.' }) {
  if (!rows || rows.length === 0) {
    return <div className="es-table-empty">{emptyMessage}</div>
  }
  return (
    <div className="es-table-wrap">
      <table className="es-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th key={col.key}>{col.header}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr key={row.id || i} onClick={() => onRowClick?.(row)} className={onRowClick ? 'es-table__row--clickable' : ''}>
              {columns.map((col) => (
                <td key={col.key}>{col.render ? col.render(row) : row[col.key]}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
