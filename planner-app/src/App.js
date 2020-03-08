import React, {Component, Suspense } from 'react'
import './App.css'
import Header from './components/Header.js'
import Login from './components/Login.js'
import { Redirect, Switch, Route } from 'react-router-dom'

class App extends Component {
  render() {
	  return (
      <div className="container">
       <Login />
      </div>
    );
}
}

export default App;
