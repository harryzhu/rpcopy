连接 Access数据库
<code>

    $objOleDbConnection = New-Object "System.Data.OleDb.OleDbConnection"
	$objOleDbCommand = New-Object "System.Data.OleDb.OleDbCommand"
	$objOleDbAdapter = New-Object "System.Data.OleDb.OleDbDataAdapter"
	$objDataTable = New-Object "System.Data.DataTable"

	$objOleDbConnection.ConnectionString = "Provider=Microsoft.Jet.OLEDB.4.0;Data Source=C:\Script\PowerShell-Example.mdb;"
	$objOleDbConnection.Open()
	
	$objOleDbConnection.State
	
	$objOleDbCommand.Connection = $objOleDbConnection
	$objOleDbCommand.CommandText = "SELECT * FROM [Example]"
	
	##set the Adapter object
	$objOleDbAdapter.SelectCommand = $objOleDbCommand
	
	##fill the objDataTable object with the results
	$objOleDbAdapter.Fill($objDataTable)
	
	##To display the “raw” contents, just enter
	$objDataTable
	
	## close the connection 
	$objOleDbConnection.Close()	

</code>