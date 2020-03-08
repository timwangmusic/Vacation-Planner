import React, {Component, Suspense, useEffect } from 'react'
import { Redirect, Switch, Route } from 'react-router-dom'

import './App.css'
import Header from './components/Header.js'
import Login from './components/Login.js'
import useForm from './components/useForm.js'

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
